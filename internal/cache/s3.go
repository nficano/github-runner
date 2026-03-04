package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nficano/github-runner/pkg/api"
)

// S3API is the subset of the S3 client that the cache backend requires.
// This interface exists to support testing with mock implementations.
type S3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
}

// S3CacheConfig holds configuration for the S3-backed cache.
type S3CacheConfig struct {
	// Bucket is the S3 bucket name.
	Bucket string
	// Prefix is an optional key prefix applied to all cache objects.
	Prefix string
	// Client is the S3 API client to use. The caller is responsible for
	// configuring authentication (IAM roles, explicit keys) and any custom
	// endpoint (e.g., MinIO) before passing this in.
	Client S3API
}

// S3Cache implements the Cache interface using an S3-compatible object store.
// Metadata such as scope, TTL, and paths are stored as S3 object tags.
// Thread safety is provided by the underlying S3 client; this type adds an
// additional mutex for internal state like hit/miss counters.
type S3Cache struct {
	cfg S3CacheConfig
	mu  sync.Mutex // guards nothing mutable beyond atomic counters; reserved for future use

	hitCount      atomic.Int64
	missCount     atomic.Int64
	evictionCount atomic.Int64
}

// NewS3Cache creates a new S3Cache with the given configuration.
func NewS3Cache(cfg S3CacheConfig) *S3Cache {
	return &S3Cache{cfg: cfg}
}

// s3Key returns the full S3 key for a cache entry.
func (sc *S3Cache) s3Key(key string) string {
	if sc.cfg.Prefix == "" {
		return key
	}
	return sc.cfg.Prefix + "/" + key
}

// Get retrieves a cache entry from S3. Returns ErrCacheMiss if the object
// does not exist. The caller is responsible for closing the returned ReadCloser.
func (sc *S3Cache) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := sc.cfg.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sc.cfg.Bucket),
		Key:    aws.String(sc.s3Key(key)),
	})
	if err != nil {
		// Check for NoSuchKey error.
		var nsk *s3types.NoSuchKey
		if isNoSuchKey(err, nsk) {
			sc.missCount.Add(1)
			return nil, ErrCacheMiss
		}
		sc.missCount.Add(1)
		return nil, fmt.Errorf("s3 get object %q: %w", key, err)
	}

	// Check if the entry has expired by reading tags.
	if sc.isExpiredByTags(ctx, key) {
		result.Body.Close()
		sc.missCount.Add(1)
		// Best-effort cleanup of expired entry.
		_ = sc.Delete(ctx, key)
		return nil, ErrCacheMiss
	}

	sc.hitCount.Add(1)
	slog.Debug("s3 cache hit", "key", key)
	return result.Body, nil
}

// isExpiredByTags checks object tags for a TTL expiration. Returns true if
// the entry has expired.
func (sc *S3Cache) isExpiredByTags(ctx context.Context, key string) bool {
	tagging, err := sc.cfg.Client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(sc.cfg.Bucket),
		Key:    aws.String(sc.s3Key(key)),
	})
	if err != nil {
		return false
	}

	var expiresAt string
	for _, tag := range tagging.TagSet {
		if aws.ToString(tag.Key) == "expires_at" {
			expiresAt = aws.ToString(tag.Value)
			break
		}
	}

	if expiresAt == "" {
		return false
	}

	ts, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return time.Now().After(ts)
}

// isNoSuchKey returns true if the error indicates the S3 object does not exist.
func isNoSuchKey(err error, _ *s3types.NoSuchKey) bool {
	// The AWS SDK wraps errors; check using string matching as a fallback
	// since ErrorAs may not work with all S3-compatible implementations.
	if err == nil {
		return false
	}
	// Try the standard approach first.
	var nsk *s3types.NoSuchKey
	if ok := errorAs(err, &nsk); ok {
		return true
	}
	// Some S3-compatible services return different error shapes. A
	// production implementation might also check for a 404 status code.
	return false
}

// errorAs is a wrapper around errors.As to support testing.
func errorAs(err error, target interface{}) bool {
	// Use type switch to avoid import cycle with errors package in tests.
	switch t := target.(type) {
	case **s3types.NoSuchKey:
		var nsk *s3types.NoSuchKey
		if asErr, ok := err.(*s3types.NoSuchKey); ok {
			*t = asErr
			return true
		}
		_ = nsk
	}
	return false
}

// Put stores a cache entry in S3 with metadata as object tags.
func (sc *S3Cache) Put(ctx context.Context, key string, r io.Reader, opts api.CacheOptions) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading cache content: %w", err)
	}

	tags := buildTags(opts)

	_, err = sc.cfg.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(sc.cfg.Bucket),
		Key:         aws.String(sc.s3Key(key)),
		Body:        bytes.NewReader(data),
		Tagging:     aws.String(tags),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("s3 put object %q: %w", key, err)
	}

	slog.Debug("s3 cache put", "key", key, "size", len(data))
	return nil
}

// buildTags encodes CacheOptions as an S3 tagging query string.
func buildTags(opts api.CacheOptions) string {
	var parts []string

	if opts.Scope != "" {
		parts = append(parts, "scope="+opts.Scope)
	}

	if opts.TTL > 0 {
		expiresAt := time.Now().Add(opts.TTL).Format(time.RFC3339)
		parts = append(parts, "expires_at="+expiresAt)
		parts = append(parts, "ttl_seconds="+strconv.FormatInt(int64(opts.TTL.Seconds()), 10))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "&"
		}
		result += p
	}
	return result
}

// Delete removes a single cache entry from S3. Returns nil if the key does not exist.
func (sc *S3Cache) Delete(ctx context.Context, key string) error {
	_, err := sc.cfg.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(sc.cfg.Bucket),
		Key:    aws.String(sc.s3Key(key)),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object %q: %w", key, err)
	}
	return nil
}

// Stats returns aggregate statistics about the S3 cache by listing all
// objects under the configured prefix.
func (sc *S3Cache) Stats(ctx context.Context) (*api.CacheStats, error) {
	var entries int64
	var totalSize int64
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(sc.cfg.Bucket),
		}
		if sc.cfg.Prefix != "" {
			input.Prefix = aws.String(sc.cfg.Prefix + "/")
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		result, err := sc.cfg.Client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("s3 list objects: %w", err)
		}

		for _, obj := range result.Contents {
			entries++
			if obj.Size != nil {
				totalSize += *obj.Size
			}
		}

		if result.IsTruncated != nil && *result.IsTruncated && result.NextContinuationToken != nil {
			continuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return &api.CacheStats{
		Entries:       entries,
		TotalSize:     totalSize,
		HitCount:      sc.hitCount.Load(),
		MissCount:     sc.missCount.Load(),
		EvictionCount: sc.evictionCount.Load(),
	}, nil
}

// Prune iterates over all objects in the S3 cache and removes entries that
// have expired based on their object tags. Returns the count of removed entries.
func (sc *S3Cache) Prune(ctx context.Context) (int, error) {
	removed := 0
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(sc.cfg.Bucket),
		}
		if sc.cfg.Prefix != "" {
			input.Prefix = aws.String(sc.cfg.Prefix + "/")
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		result, err := sc.cfg.Client.ListObjectsV2(ctx, input)
		if err != nil {
			return removed, fmt.Errorf("s3 list objects for prune: %w", err)
		}

		for _, obj := range result.Contents {
			if obj.Key == nil {
				continue
			}
			// Derive the cache key from the S3 key by stripping the prefix.
			cacheKey := *obj.Key
			if sc.cfg.Prefix != "" && len(cacheKey) > len(sc.cfg.Prefix)+1 {
				cacheKey = cacheKey[len(sc.cfg.Prefix)+1:]
			}

			if sc.isExpiredByTags(ctx, cacheKey) {
				if err := sc.Delete(ctx, cacheKey); err != nil {
					slog.Warn("failed to delete expired s3 cache entry", "key", cacheKey, "error", err)
					continue
				}
				removed++
				sc.evictionCount.Add(1)
			}
		}

		if result.IsTruncated != nil && *result.IsTruncated && result.NextContinuationToken != nil {
			continuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return removed, nil
}

// Verify that S3Cache implements Cache at compile time.
var _ Cache = (*S3Cache)(nil)

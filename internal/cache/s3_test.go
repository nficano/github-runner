package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/org/github-runner/pkg/api"
)

// mockObject represents an object stored in the mock S3 backend.
type mockObject struct {
	data    []byte
	tags    []s3types.Tag
	tagging string
}

// mockS3Client is a simple in-memory mock of the S3API interface.
type mockS3Client struct {
	mu      sync.Mutex
	objects map[string]*mockObject
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{
		objects: make(map[string]*mockObject),
	}
}

func (m *mockS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	obj, ok := m.objects[key]
	if !ok {
		return nil, &s3types.NoSuchKey{Message: aws.String("not found")}
	}

	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(obj.data)),
		ContentLength: aws.Int64(int64(len(obj.data))),
	}, nil
}

func (m *mockS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	obj := &mockObject{
		data: data,
	}

	// Parse tagging string into tags.
	if input.Tagging != nil {
		obj.tagging = aws.ToString(input.Tagging)
		obj.tags = parseTagging(obj.tagging)
	}

	m.objects[key] = obj
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	delete(m.objects, key)
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	prefix := aws.ToString(input.Prefix)
	var contents []s3types.Object

	for key, obj := range m.objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			size := int64(len(obj.data))
			contents = append(contents, s3types.Object{
				Key:  aws.String(key),
				Size: &size,
			})
		}
	}

	truncated := false
	return &s3.ListObjectsV2Output{
		Contents:    contents,
		IsTruncated: &truncated,
	}, nil
}

func (m *mockS3Client) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	obj, ok := m.objects[key]
	if !ok {
		return nil, &s3types.NoSuchKey{Message: aws.String("not found")}
	}

	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(obj.data))),
	}, nil
}

func (m *mockS3Client) GetObjectTagging(_ context.Context, input *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	obj, ok := m.objects[key]
	if !ok {
		return nil, &s3types.NoSuchKey{Message: aws.String("not found")}
	}

	return &s3.GetObjectTaggingOutput{
		TagSet: obj.tags,
	}, nil
}

func (m *mockS3Client) PutObjectTagging(_ context.Context, input *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	obj, ok := m.objects[key]
	if !ok {
		return nil, &s3types.NoSuchKey{Message: aws.String("not found")}
	}

	if input.Tagging != nil {
		obj.tags = input.Tagging.TagSet
	}

	return &s3.PutObjectTaggingOutput{}, nil
}

// parseTagging parses an S3 tagging query string like "key1=val1&key2=val2".
func parseTagging(s string) []s3types.Tag {
	if s == "" {
		return nil
	}
	var tags []s3types.Tag
	for _, pair := range strings.Split(s, "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			tags = append(tags, s3types.Tag{
				Key:   aws.String(parts[0]),
				Value: aws.String(parts[1]),
			})
		}
	}
	return tags
}

func TestS3Cache(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func(t *testing.T, sc *S3Cache)
	}{
		{
			name: "put and get",
			fn: func(t *testing.T, sc *S3Cache) {
				data := []byte("hello s3 cache")
				err := sc.Put(ctx, "key1", bytes.NewReader(data), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				rc, err := sc.Get(ctx, "key1")
				if err != nil {
					t.Fatalf("Get: %v", err)
				}
				defer rc.Close()

				got, err := io.ReadAll(rc)
				if err != nil {
					t.Fatalf("ReadAll: %v", err)
				}
				if !bytes.Equal(got, data) {
					t.Errorf("got %q, want %q", got, data)
				}
			},
		},
		{
			name: "get miss returns ErrCacheMiss",
			fn: func(t *testing.T, sc *S3Cache) {
				_, err := sc.Get(ctx, "nonexistent")
				if err != ErrCacheMiss {
					t.Errorf("got error %v, want ErrCacheMiss", err)
				}
			},
		},
		{
			name: "delete existing entry",
			fn: func(t *testing.T, sc *S3Cache) {
				_ = sc.Put(ctx, "key1", bytes.NewReader([]byte("data")), api.CacheOptions{})

				err := sc.Delete(ctx, "key1")
				if err != nil {
					t.Fatalf("Delete: %v", err)
				}

				_, err = sc.Get(ctx, "key1")
				if err != ErrCacheMiss {
					t.Errorf("after delete: got %v, want ErrCacheMiss", err)
				}
			},
		},
		{
			name: "delete nonexistent key is no-op",
			fn: func(t *testing.T, sc *S3Cache) {
				err := sc.Delete(ctx, "nonexistent")
				if err != nil {
					t.Errorf("Delete nonexistent: %v", err)
				}
			},
		},
		{
			name: "stats counts objects and sizes",
			fn: func(t *testing.T, sc *S3Cache) {
				_ = sc.Put(ctx, "a", bytes.NewReader([]byte("aaa")), api.CacheOptions{})
				_ = sc.Put(ctx, "b", bytes.NewReader([]byte("bb")), api.CacheOptions{})

				stats, err := sc.Stats(ctx)
				if err != nil {
					t.Fatalf("Stats: %v", err)
				}

				if stats.Entries != 2 {
					t.Errorf("entries = %d, want 2", stats.Entries)
				}
				if stats.TotalSize != 5 {
					t.Errorf("total_size = %d, want 5", stats.TotalSize)
				}
			},
		},
		{
			name: "hit and miss counts",
			fn: func(t *testing.T, sc *S3Cache) {
				_ = sc.Put(ctx, "key1", bytes.NewReader([]byte("data")), api.CacheOptions{})

				rc, _ := sc.Get(ctx, "key1")
				rc.Close()
				_, _ = sc.Get(ctx, "miss1")

				stats, _ := sc.Stats(ctx)
				if stats.HitCount != 1 {
					t.Errorf("hit_count = %d, want 1", stats.HitCount)
				}
				if stats.MissCount != 1 {
					t.Errorf("miss_count = %d, want 1", stats.MissCount)
				}
			},
		},
		{
			name: "prefix is applied to keys",
			fn: func(t *testing.T, sc *S3Cache) {
				sc.cfg.Prefix = "runner-cache"

				_ = sc.Put(ctx, "key1", bytes.NewReader([]byte("data")), api.CacheOptions{})

				// The mock should have stored it under the prefixed key.
				mock := sc.cfg.Client.(*mockS3Client)
				mock.mu.Lock()
				_, ok := mock.objects["runner-cache/key1"]
				mock.mu.Unlock()

				if !ok {
					t.Error("expected object stored under prefixed key")
				}
			},
		},
		{
			name: "put with scope tag",
			fn: func(t *testing.T, sc *S3Cache) {
				opts := api.CacheOptions{Scope: "refs/heads/main"}
				_ = sc.Put(ctx, "scoped", bytes.NewReader([]byte("data")), opts)

				mock := sc.cfg.Client.(*mockS3Client)
				mock.mu.Lock()
				obj := mock.objects["scoped"]
				mock.mu.Unlock()

				found := false
				for _, tag := range obj.tags {
					if aws.ToString(tag.Key) == "scope" && aws.ToString(tag.Value) == "refs/heads/main" {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected scope tag on object")
				}
			},
		},
		{
			name: "TTL expired entries are pruned",
			fn: func(t *testing.T, sc *S3Cache) {
				// Put with a very short TTL (already expired by the time we check).
				opts := api.CacheOptions{TTL: time.Millisecond}
				_ = sc.Put(ctx, "expiring", bytes.NewReader([]byte("temp")), opts)

				// Manually adjust the expires_at tag to be in the past.
				mock := sc.cfg.Client.(*mockS3Client)
				mock.mu.Lock()
				obj := mock.objects["expiring"]
				for i, tag := range obj.tags {
					if aws.ToString(tag.Key) == "expires_at" {
						past := time.Now().Add(-time.Hour).Format(time.RFC3339)
						obj.tags[i] = s3types.Tag{
							Key:   aws.String("expires_at"),
							Value: aws.String(past),
						}
					}
				}
				mock.mu.Unlock()

				removed, err := sc.Prune(ctx)
				if err != nil {
					t.Fatalf("Prune: %v", err)
				}
				if removed != 1 {
					t.Errorf("removed = %d, want 1", removed)
				}
			},
		},
		{
			name: "put replaces existing entry",
			fn: func(t *testing.T, sc *S3Cache) {
				_ = sc.Put(ctx, "key1", bytes.NewReader([]byte("v1")), api.CacheOptions{})
				_ = sc.Put(ctx, "key1", bytes.NewReader([]byte("v2")), api.CacheOptions{})

				rc, err := sc.Get(ctx, "key1")
				if err != nil {
					t.Fatalf("Get: %v", err)
				}
				defer rc.Close()

				got, _ := io.ReadAll(rc)
				if string(got) != "v2" {
					t.Errorf("got %q, want %q", got, "v2")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockS3Client()
			sc := NewS3Cache(S3CacheConfig{
				Bucket: "test-bucket",
				Client: mock,
			})
			tt.fn(t, sc)
		})
	}
}

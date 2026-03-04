// Package cache defines the Cache interface for storing and retrieving
// step-level cache entries used to speed up workflow runs.
package cache

import (
	"context"
	"errors"
	"io"

	"github.com/nficano/github-runner/pkg/api"
)

// ErrCacheMiss is returned by Get when the requested key is not present in
// the cache and no restore key prefix matched.
var ErrCacheMiss = errors.New("cache miss")

// Cache abstracts the storage backend used for workflow caching. Every
// implementation must be safe for concurrent use.
type Cache interface {
	// Get looks up a cache entry by its exact key. If the key is not found
	// the implementation should return ErrCacheMiss. The caller is
	// responsible for closing the returned ReadCloser.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Put stores a cache entry under the given key, reading the content
	// from r. If an entry with the same key already exists it is replaced.
	// The CacheOptions control TTL, scope, and associated paths metadata.
	Put(ctx context.Context, key string, r io.Reader, opts api.CacheOptions) error

	// Delete removes a single cache entry. It returns nil if the key does
	// not exist.
	Delete(ctx context.Context, key string) error

	// Stats returns aggregate statistics about the cache backend.
	Stats(ctx context.Context) (*api.CacheStats, error)

	// Prune removes expired or least-recently-used entries to reclaim
	// space. It returns the number of entries removed.
	Prune(ctx context.Context) (int, error)
}

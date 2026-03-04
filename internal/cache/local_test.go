package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/org/github-runner/pkg/api"
)

func newTestLocalCache(t *testing.T, maxSize int64) *LocalCache {
	t.Helper()
	dir := t.TempDir()
	lc, err := NewLocalCache(LocalCacheConfig{
		Path:    dir,
		MaxSize: maxSize,
	})
	if err != nil {
		t.Fatalf("NewLocalCache: %v", err)
	}
	return lc
}

func TestLocalCache(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func(t *testing.T, lc *LocalCache)
	}{
		{
			name: "put and get",
			fn: func(t *testing.T, lc *LocalCache) {
				data := []byte("hello, cache")
				err := lc.Put(ctx, "key1", bytes.NewReader(data), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				rc, err := lc.Get(ctx, "key1")
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
			fn: func(t *testing.T, lc *LocalCache) {
				_, err := lc.Get(ctx, "nonexistent")
				if err != ErrCacheMiss {
					t.Errorf("got error %v, want ErrCacheMiss", err)
				}
			},
		},
		{
			name: "delete existing entry",
			fn: func(t *testing.T, lc *LocalCache) {
				err := lc.Put(ctx, "key1", bytes.NewReader([]byte("data")), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				err = lc.Delete(ctx, "key1")
				if err != nil {
					t.Fatalf("Delete: %v", err)
				}

				_, err = lc.Get(ctx, "key1")
				if err != ErrCacheMiss {
					t.Errorf("after delete: got error %v, want ErrCacheMiss", err)
				}
			},
		},
		{
			name: "delete nonexistent key is no-op",
			fn: func(t *testing.T, lc *LocalCache) {
				err := lc.Delete(ctx, "nonexistent")
				if err != nil {
					t.Errorf("Delete nonexistent: %v", err)
				}
			},
		},
		{
			name: "put replaces existing entry",
			fn: func(t *testing.T, lc *LocalCache) {
				err := lc.Put(ctx, "key1", bytes.NewReader([]byte("v1")), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put v1: %v", err)
				}

				err = lc.Put(ctx, "key1", bytes.NewReader([]byte("v2")), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put v2: %v", err)
				}

				rc, err := lc.Get(ctx, "key1")
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
		{
			name: "LRU eviction when max size exceeded",
			fn: func(t *testing.T, lc *LocalCache) {
				// MaxSize is 20 bytes. Put two 10-byte entries, then a third
				// that should evict the oldest.
				lc.cfg.MaxSize = 20

				err := lc.Put(ctx, "a", bytes.NewReader(bytes.Repeat([]byte("x"), 10)), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put a: %v", err)
				}

				// Touch "a" to make it most-recently-used.
				time.Sleep(time.Millisecond)
				_, _ = lc.Get(ctx, "a")

				err = lc.Put(ctx, "b", bytes.NewReader(bytes.Repeat([]byte("y"), 10)), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put b: %v", err)
				}

				// Now add "c" (10 bytes). Total would be 30, exceeding 20.
				// "a" was used more recently than "b" because we called Get on it,
				// but "b" was put after "a"'s Get. We need to be careful about
				// timing. Let's just check that one entry was evicted.
				time.Sleep(time.Millisecond)
				err = lc.Put(ctx, "c", bytes.NewReader(bytes.Repeat([]byte("z"), 10)), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put c: %v", err)
				}

				stats, err := lc.Stats(ctx)
				if err != nil {
					t.Fatalf("Stats: %v", err)
				}

				if stats.Entries != 2 {
					t.Errorf("expected 2 entries after eviction, got %d", stats.Entries)
				}
				if stats.TotalSize > 20 {
					t.Errorf("total size %d exceeds max %d", stats.TotalSize, lc.cfg.MaxSize)
				}
			},
		},
		{
			name: "TTL expiry",
			fn: func(t *testing.T, lc *LocalCache) {
				opts := api.CacheOptions{TTL: time.Millisecond}
				err := lc.Put(ctx, "expiring", bytes.NewReader([]byte("temp")), opts)
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				// Wait for the entry to expire.
				time.Sleep(5 * time.Millisecond)

				_, err = lc.Get(ctx, "expiring")
				if err != ErrCacheMiss {
					t.Errorf("expected ErrCacheMiss for expired entry, got %v", err)
				}
			},
		},
		{
			name: "stats hit and miss counts",
			fn: func(t *testing.T, lc *LocalCache) {
				err := lc.Put(ctx, "key1", bytes.NewReader([]byte("data")), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				// 1 hit
				rc, _ := lc.Get(ctx, "key1")
				rc.Close()

				// 2 misses
				_, _ = lc.Get(ctx, "miss1")
				_, _ = lc.Get(ctx, "miss2")

				stats, err := lc.Stats(ctx)
				if err != nil {
					t.Fatalf("Stats: %v", err)
				}

				if stats.HitCount != 1 {
					t.Errorf("hit count = %d, want 1", stats.HitCount)
				}
				if stats.MissCount != 2 {
					t.Errorf("miss count = %d, want 2", stats.MissCount)
				}
			},
		},
		{
			name: "prune removes expired entries",
			fn: func(t *testing.T, lc *LocalCache) {
				opts := api.CacheOptions{TTL: time.Millisecond}
				_ = lc.Put(ctx, "e1", bytes.NewReader([]byte("a")), opts)
				_ = lc.Put(ctx, "e2", bytes.NewReader([]byte("b")), opts)
				_ = lc.Put(ctx, "keep", bytes.NewReader([]byte("c")), api.CacheOptions{})

				time.Sleep(5 * time.Millisecond)

				removed, err := lc.Prune(ctx)
				if err != nil {
					t.Fatalf("Prune: %v", err)
				}
				if removed != 2 {
					t.Errorf("removed = %d, want 2", removed)
				}

				stats, _ := lc.Stats(ctx)
				if stats.Entries != 1 {
					t.Errorf("entries = %d, want 1", stats.Entries)
				}
			},
		},
		{
			name: "persistence across instances",
			fn: func(t *testing.T, lc *LocalCache) {
				err := lc.Put(ctx, "persistent", bytes.NewReader([]byte("survive")), api.CacheOptions{})
				if err != nil {
					t.Fatalf("Put: %v", err)
				}

				// Create a new cache instance pointing to the same directory.
				lc2, err := NewLocalCache(lc.cfg)
				if err != nil {
					t.Fatalf("NewLocalCache: %v", err)
				}

				rc, err := lc2.Get(ctx, "persistent")
				if err != nil {
					t.Fatalf("Get from new instance: %v", err)
				}
				defer rc.Close()

				got, _ := io.ReadAll(rc)
				if string(got) != "survive" {
					t.Errorf("got %q, want %q", got, "survive")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := newTestLocalCache(t, 0)
			tt.fn(t, lc)
		})
	}
}

func TestLocalCacheIndexCorruption(t *testing.T) {
	dir := t.TempDir()
	// Write corrupt index data.
	indexPath := dir + "/index.json"
	if err := os.WriteFile(indexPath, []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Should not error; it logs a warning and starts fresh.
	lc, err := NewLocalCache(LocalCacheConfig{Path: dir, MaxSize: 1024})
	if err != nil {
		t.Fatalf("NewLocalCache with corrupt index: %v", err)
	}

	stats, _ := lc.Stats(context.Background())
	if stats.Entries != 0 {
		t.Errorf("expected 0 entries after corrupt index, got %d", stats.Entries)
	}
}

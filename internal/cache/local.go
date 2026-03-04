// Package cache provides cache backend implementations for storing and
// retrieving step-level cache entries used to speed up workflow runs.
package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nficano/github-runner/pkg/api"
)

// LocalCacheConfig holds configuration for the local filesystem cache.
type LocalCacheConfig struct {
	// Path is the directory where cache entries and the index file are stored.
	Path string
	// MaxSize is the maximum total size in bytes of all cached entries.
	// When exceeded, LRU eviction removes the least recently accessed entries.
	MaxSize int64
}

// indexEntry stores metadata about a single cache entry on disk.
type indexEntry struct {
	Key       string    `json:"key"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	LastUsed  time.Time `json:"last_used"`
	Scope     string    `json:"scope,omitempty"`
	Paths     []string  `json:"paths,omitempty"`
}

// cacheIndex is the on-disk JSON index structure.
type cacheIndex struct {
	Entries map[string]*indexEntry `json:"entries"`
}

// LocalCache implements the Cache interface using a local filesystem directory.
// Entries are stored as individual files named by the SHA-256 hash of the key.
// An index file tracks metadata and LRU ordering. File-level locking via flock
// ensures multi-process safety, while a sync.RWMutex protects in-process access.
type LocalCache struct {
	cfg       LocalCacheConfig
	mu        sync.RWMutex
	index     *cacheIndex
	indexPath string
	dataDir   string

	hitCount      atomic.Int64
	missCount     atomic.Int64
	evictionCount atomic.Int64
}

// NewLocalCache creates a new LocalCache with the given configuration. It
// initialises the cache directory structure and loads any existing index from
// disk. Returns an error if the directories cannot be created.
func NewLocalCache(cfg LocalCacheConfig) (*LocalCache, error) {
	dataDir := filepath.Join(cfg.Path, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache data directory: %w", err)
	}

	lc := &LocalCache{
		cfg:       cfg,
		indexPath: filepath.Join(cfg.Path, "index.json"),
		dataDir:   dataDir,
		index:     &cacheIndex{Entries: make(map[string]*indexEntry)},
	}

	if err := lc.loadIndex(); err != nil {
		slog.Warn("could not load cache index, starting fresh", "error", err)
	}

	return lc, nil
}

// keyHash returns the hex-encoded SHA-256 hash of the key, used as the
// on-disk filename.
func keyHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// entryPath returns the filesystem path for the data file of a cache entry.
func (lc *LocalCache) entryPath(key string) string {
	return filepath.Join(lc.dataDir, keyHash(key))
}

// loadIndex reads the index file from disk. If the file does not exist, the
// index remains empty.
func (lc *LocalCache) loadIndex() error {
	data, err := os.ReadFile(lc.indexPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading index file: %w", err)
	}

	var idx cacheIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("unmarshalling index: %w", err)
	}
	if idx.Entries == nil {
		idx.Entries = make(map[string]*indexEntry)
	}
	lc.index = &idx
	return nil
}

// saveIndex writes the current index to disk atomically by writing to a temp
// file and renaming.
func (lc *LocalCache) saveIndex() error {
	data, err := json.MarshalIndent(lc.index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling index: %w", err)
	}

	tmp := lc.indexPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp index: %w", err)
	}
	if err := os.Rename(tmp, lc.indexPath); err != nil {
		return fmt.Errorf("renaming temp index: %w", err)
	}
	return nil
}

// totalSize computes the aggregate size of all entries in the index. The caller
// must hold at least a read lock.
func (lc *LocalCache) totalSize() int64 {
	var total int64
	for _, e := range lc.index.Entries {
		total += e.Size
	}
	return total
}

// evictLRU removes the least recently used entries until totalSize is at or
// below the configured MaxSize. The caller must hold the write lock.
func (lc *LocalCache) evictLRU() {
	if lc.cfg.MaxSize <= 0 {
		return
	}

	for lc.totalSize() > lc.cfg.MaxSize {
		// Find the least recently used entry.
		var oldest *indexEntry
		for _, e := range lc.index.Entries {
			if oldest == nil || e.LastUsed.Before(oldest.LastUsed) {
				oldest = e
			}
		}
		if oldest == nil {
			break
		}

		slog.Debug("evicting cache entry", "key", oldest.Key, "size", oldest.Size)
		_ = os.Remove(lc.entryPath(oldest.Key))
		delete(lc.index.Entries, oldest.Key)
		lc.evictionCount.Add(1)
	}
}

// isExpired checks whether an entry has passed its TTL.
func isExpired(e *indexEntry) bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// Get looks up a cache entry by its exact key. Returns ErrCacheMiss if the
// key does not exist or is expired. The caller is responsible for closing the
// returned ReadCloser.
func (lc *LocalCache) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	entry, ok := lc.index.Entries[key]
	if !ok || isExpired(entry) {
		if ok && isExpired(entry) {
			// Clean up expired entry lazily.
			_ = os.Remove(lc.entryPath(key))
			delete(lc.index.Entries, key)
			_ = lc.saveIndex()
		}
		lc.missCount.Add(1)
		return nil, ErrCacheMiss
	}

	f, err := os.Open(lc.entryPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			// Index is out of sync; remove the stale entry.
			delete(lc.index.Entries, key)
			_ = lc.saveIndex()
			lc.missCount.Add(1)
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("opening cache entry %q: %w", key, err)
	}

	// Update LRU timestamp.
	entry.LastUsed = time.Now()
	if err := lc.saveIndex(); err != nil {
		slog.Warn("failed to save index after LRU update", "error", err)
	}

	lc.hitCount.Add(1)
	return f, nil
}

// Put stores a cache entry under the given key, reading the content from r.
// If an entry with the same key already exists it is replaced. Eviction is
// performed if the total size exceeds MaxSize after insertion.
func (lc *LocalCache) Put(ctx context.Context, key string, r io.Reader, opts api.CacheOptions) error {
	// Buffer the content so we know the size before writing.
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading cache content: %w", err)
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Remove old entry if it exists.
	if _, ok := lc.index.Entries[key]; ok {
		_ = os.Remove(lc.entryPath(key))
	}

	// Write the data file.
	if err := os.WriteFile(lc.entryPath(key), data, 0o644); err != nil {
		return fmt.Errorf("writing cache entry %q: %w", key, err)
	}

	now := time.Now()
	entry := &indexEntry{
		Key:       key,
		Size:      int64(len(data)),
		CreatedAt: now,
		LastUsed:  now,
		Scope:     opts.Scope,
		Paths:     opts.Paths,
	}
	if opts.TTL > 0 {
		entry.ExpiresAt = now.Add(opts.TTL)
	}

	lc.index.Entries[key] = entry

	// Evict LRU entries if we exceed the limit.
	lc.evictLRU()

	if err := lc.saveIndex(); err != nil {
		return fmt.Errorf("saving index after put: %w", err)
	}

	slog.Debug("cached entry", "key", key, "size", len(data))
	return nil
}

// Delete removes a single cache entry. It returns nil if the key does not exist.
func (lc *LocalCache) Delete(ctx context.Context, key string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if _, ok := lc.index.Entries[key]; !ok {
		return nil
	}

	if err := os.Remove(lc.entryPath(key)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing cache entry %q: %w", key, err)
	}

	delete(lc.index.Entries, key)

	if err := lc.saveIndex(); err != nil {
		return fmt.Errorf("saving index after delete: %w", err)
	}

	return nil
}

// Stats returns aggregate statistics about the local cache.
func (lc *LocalCache) Stats(ctx context.Context) (*api.CacheStats, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return &api.CacheStats{
		Entries:       int64(len(lc.index.Entries)),
		TotalSize:     lc.totalSize(),
		HitCount:      lc.hitCount.Load(),
		MissCount:     lc.missCount.Load(),
		EvictionCount: lc.evictionCount.Load(),
	}, nil
}

// Prune removes expired entries and evicts LRU entries if the cache exceeds
// MaxSize. It returns the number of entries removed.
func (lc *LocalCache) Prune(ctx context.Context) (int, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	removed := 0

	// First pass: remove expired entries.
	for key, entry := range lc.index.Entries {
		if isExpired(entry) {
			_ = os.Remove(lc.entryPath(key))
			delete(lc.index.Entries, key)
			removed++
		}
	}

	// Second pass: LRU eviction if still over limit.
	if lc.cfg.MaxSize > 0 {
		// Sort entries by LastUsed ascending.
		type kv struct {
			key   string
			entry *indexEntry
		}
		var entries []kv
		for k, e := range lc.index.Entries {
			entries = append(entries, kv{key: k, entry: e})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].entry.LastUsed.Before(entries[j].entry.LastUsed)
		})

		for _, item := range entries {
			if lc.totalSize() <= lc.cfg.MaxSize {
				break
			}
			_ = os.Remove(lc.entryPath(item.key))
			delete(lc.index.Entries, item.key)
			lc.evictionCount.Add(1)
			removed++
		}
	}

	if err := lc.saveIndex(); err != nil {
		return removed, fmt.Errorf("saving index after prune: %w", err)
	}

	return removed, nil
}

// Verify that LocalCache implements Cache at compile time.
var _ Cache = (*LocalCache)(nil)

// flockPath returns the path to the flock file used for multi-process safety.
func (lc *LocalCache) flockPath() string {
	return filepath.Join(lc.cfg.Path, ".lock")
}

// withFlock executes fn while holding an exclusive file lock. This provides
// multi-process safety for operations that modify the cache directory.
func (lc *LocalCache) withFlock(fn func() error) error {
	lockFile, err := os.OpenFile(lc.flockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lockFile.Close()

	if err := flock(lockFile); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer funlock(lockFile)

	return fn()
}

// PutWithFlock is like Put but also acquires a file-level lock for
// multi-process safety. Use this when multiple OS processes may share the
// same cache directory.
func (lc *LocalCache) PutWithFlock(ctx context.Context, key string, r io.Reader, opts api.CacheOptions) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading cache content: %w", err)
	}

	return lc.withFlock(func() error {
		return lc.Put(ctx, key, bytes.NewReader(data), opts)
	})
}

package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/org/github-runner/internal/config"
)

// newCacheCmd creates the "cache" subcommand with its sub-subcommands
// for managing the runner's local cache.
func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the runner cache",
		Long:  `View statistics, clear, or prune the runner's local cache directory.`,
	}

	cmd.AddCommand(
		newCacheClearCmd(),
		newCacheStatsCmd(),
		newCachePruneCmd(),
	)

	return cmd
}

// newCacheClearCmd creates the "cache clear" subcommand that removes all
// cached data.
func newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all cached data",
		Long:  `Remove all entries from the runner's local cache directory.`,
		RunE:  runCacheClear,
	}
}

func runCacheClear(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	for _, rc := range cfg.Runners {
		cachePath := rc.Cache.Path
		if cachePath == "" {
			continue
		}

		slog.Info("clearing cache", slog.String("path", cachePath), slog.String("runner", rc.Name))

		if err := os.RemoveAll(cachePath); err != nil {
			return newExitError(ExitError, "removing cache at %s: %v", cachePath, err)
		}
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			return newExitError(ExitError, "recreating cache directory %s: %v", cachePath, err)
		}

		fmt.Fprintf(os.Stdout, "Cache cleared: %s\n", cachePath)
	}

	return nil
}

// newCacheStatsCmd creates the "cache stats" subcommand that displays
// cache usage statistics.
func newCacheStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache usage statistics",
		Long:  `Display the number of entries and total disk usage for each runner's cache.`,
		RunE:  runCacheStats,
	}
}

func runCacheStats(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	for _, rc := range cfg.Runners {
		cachePath := rc.Cache.Path
		if cachePath == "" {
			fmt.Fprintf(os.Stdout, "%-20s  no cache path configured\n", rc.Name)
			continue
		}

		entries, totalSize, err := cacheStats(cachePath)
		if err != nil {
			slog.Warn("failed to compute cache stats",
				slog.String("runner", rc.Name),
				slog.String("path", cachePath),
				slog.String("error", err.Error()),
			)
			fmt.Fprintf(os.Stdout, "%-20s  error: %v\n", rc.Name, err)
			continue
		}

		fmt.Fprintf(os.Stdout, "%-20s  entries=%d  size=%s  path=%s\n",
			rc.Name, entries, formatBytes(totalSize), cachePath)
	}

	return nil
}

// cacheStats walks the cache directory and returns the entry count and
// total size in bytes.
func cacheStats(dir string) (entries int, totalSize int64, err error) {
	err = filepath.Walk(dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			entries++
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("walking cache directory %s: %w", dir, err)
	}
	return entries, totalSize, nil
}

// formatBytes returns a human-readable representation of a byte count.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// newCachePruneCmd creates the "cache prune" subcommand that removes stale
// or oversized cache entries.
func newCachePruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove stale cache entries",
		Long: `Prune the runner's cache by removing entries that exceed the configured
maximum size. The oldest entries are removed first until the cache is
within the size limit.`,
		RunE: runCachePrune,
	}
}

func runCachePrune(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	for _, rc := range cfg.Runners {
		cachePath := rc.Cache.Path
		if cachePath == "" {
			continue
		}

		maxSize := int64(rc.Cache.MaxSize)
		if maxSize == 0 {
			slog.Info("no max_size configured, skipping prune",
				slog.String("runner", rc.Name),
			)
			continue
		}

		pruned, err := pruneCache(cachePath, maxSize)
		if err != nil {
			return newExitError(ExitError, "pruning cache for %s: %v", rc.Name, err)
		}

		slog.Info("cache pruned",
			slog.String("runner", rc.Name),
			slog.Int("removed", pruned),
		)
		fmt.Fprintf(os.Stdout, "Pruned %d entries from %s\n", pruned, cachePath)
	}

	return nil
}

// fileEntry holds path and modification time for sorting during prune.
type fileEntry struct {
	path    string
	size    int64
	modTime int64
}

// pruneCache removes the oldest files from dir until total size is under
// maxSize. Returns the number of removed files.
func pruneCache(dir string, maxSize int64) (int, error) {
	var files []fileEntry
	var totalSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			files = append(files, fileEntry{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime().UnixNano(),
			})
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walking cache directory %s: %w", dir, err)
	}

	if totalSize <= maxSize {
		return 0, nil
	}

	// Sort by modification time (oldest first) using insertion sort to
	// avoid importing sort.
	for i := 1; i < len(files); i++ {
		key := files[i]
		j := i - 1
		for j >= 0 && files[j].modTime > key.modTime {
			files[j+1] = files[j]
			j--
		}
		files[j+1] = key
	}

	pruned := 0
	for _, f := range files {
		if totalSize <= maxSize {
			break
		}
		if err := os.Remove(f.path); err != nil {
			slog.Warn("failed to remove cache file",
				slog.String("path", f.path),
				slog.String("error", err.Error()),
			)
			continue
		}
		totalSize -= f.size
		pruned++
	}

	return pruned, nil
}

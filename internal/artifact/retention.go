package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// EnforceRetention scans the artifact directory tree rooted at baseDir and
// removes any artifacts whose creation time plus maxAge has elapsed. It
// returns the count of removed artifacts.
//
// The directory structure is expected to be:
//
//	baseDir/<jobID>/<name>.gz        (compressed data)
//	baseDir/<jobID>/<name>.meta.json (metadata)
//
// Removal decisions are based on the ExpiresAt field in the metadata file
// when present, falling back to CreatedAt + maxAge otherwise. If maxAge is
// zero, only artifacts with an explicit ExpiresAt in the past are removed.
func EnforceRetention(ctx context.Context, baseDir string, maxAge time.Duration) (int, error) {
	removed := 0

	jobDirs, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading artifact base directory: %w", err)
	}

	now := time.Now()

	for _, jobEntry := range jobDirs {
		if !jobEntry.IsDir() {
			continue
		}

		if err := ctx.Err(); err != nil {
			return removed, fmt.Errorf("retention scan cancelled: %w", err)
		}

		jobDir := filepath.Join(baseDir, jobEntry.Name())
		files, err := os.ReadDir(jobDir)
		if err != nil {
			slog.Warn("failed to read job directory", "dir", jobDir, "error", err)
			continue
		}

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
				continue
			}

			metaPath := filepath.Join(jobDir, f.Name())
			data, err := os.ReadFile(metaPath)
			if err != nil {
				slog.Warn("failed to read artifact metadata", "path", metaPath, "error", err)
				continue
			}

			var record artifactRecord
			if err := json.Unmarshal(data, &record); err != nil {
				slog.Warn("failed to parse artifact metadata", "path", metaPath, "error", err)
				continue
			}

			expired := false
			if !record.ExpiresAt.IsZero() {
				expired = now.After(record.ExpiresAt)
			} else if maxAge > 0 {
				expired = now.After(record.CreatedAt.Add(maxAge))
			}

			if !expired {
				continue
			}

			slog.Debug("removing expired artifact",
				"job_id", jobEntry.Name(),
				"name", record.Name,
				"created_at", record.CreatedAt,
				"expires_at", record.ExpiresAt,
			)

			// Remove the data file (name.gz).
			dataPath := filepath.Join(jobDir, record.Name+".gz")
			if err := os.Remove(dataPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to remove artifact data", "path", dataPath, "error", err)
			}

			// Remove the metadata file.
			if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to remove artifact metadata", "path", metaPath, "error", err)
			}

			removed++
		}

		// Try to remove the job directory if it is now empty.
		_ = os.Remove(jobDir)
	}

	return removed, nil
}

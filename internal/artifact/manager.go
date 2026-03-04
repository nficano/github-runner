package artifact

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nficano/github-runner/pkg/api"
)

// ManagerConfig holds configuration for the local artifact manager.
type ManagerConfig struct {
	// BaseDir is the root directory where artifacts are stored, organised
	// as BaseDir/<jobID>/<name>.
	BaseDir string

	// DefaultRetentionDays is the retention period applied when the upload
	// options do not specify one. Zero means artifacts are kept indefinitely.
	DefaultRetentionDays int
}

// artifactRecord is the on-disk metadata stored alongside each artifact.
type artifactRecord struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// Manager implements the ArtifactStore interface using the local filesystem.
// Artifacts are gzip-compressed before storage and organised by job ID and
// artifact name. Thread safety is provided via sync.RWMutex.
type Manager struct {
	cfg ManagerConfig
	mu  sync.RWMutex
}

// NewManager creates a new artifact Manager with the given configuration.
// It creates the base directory if it does not exist.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if err := os.MkdirAll(cfg.BaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating artifact base directory: %w", err)
	}
	return &Manager{cfg: cfg}, nil
}

// jobDir returns the directory path for a specific job's artifacts.
func (m *Manager) jobDir(jobID string) string {
	return filepath.Join(m.cfg.BaseDir, jobID)
}

// dataPath returns the path to the compressed artifact data file.
func (m *Manager) dataPath(jobID, name string) string {
	return filepath.Join(m.jobDir(jobID), name+".gz")
}

// metaPath returns the path to the artifact metadata JSON file.
func (m *Manager) metaPath(jobID, name string) string {
	return filepath.Join(m.jobDir(jobID), name+".meta.json")
}

// Upload stores an artifact under the given job and name, reading content
// from r. The content is gzip-compressed before writing to disk. If an
// artifact with the same name already exists for the job it is replaced.
func (m *Manager) Upload(ctx context.Context, jobID string, name string, r io.Reader, opts api.UploadOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.jobDir(jobID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating job artifact directory: %w", err)
	}

	// Create the data file for compressed output.
	dataFile, err := os.Create(m.dataPath(jobID, name))
	if err != nil {
		return fmt.Errorf("creating artifact data file: %w", err)
	}
	defer dataFile.Close()

	// Determine compression level.
	level := gzip.DefaultCompression
	if opts.CompressionLevel > 0 && opts.CompressionLevel <= 9 {
		level = opts.CompressionLevel
	}

	gw, err := gzip.NewWriterLevel(dataFile, level)
	if err != nil {
		return fmt.Errorf("creating gzip writer: %w", err)
	}

	// Hash the uncompressed data while writing compressed output.
	h := sha256.New()
	tee := io.TeeReader(r, h)

	size, err := io.Copy(gw, tee)
	if err != nil {
		gw.Close()
		return fmt.Errorf("writing artifact data: %w", err)
	}

	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	digest := hex.EncodeToString(h.Sum(nil))

	// Compute expiration.
	now := time.Now()
	retentionDays := opts.RetentionDays
	if retentionDays <= 0 {
		retentionDays = m.cfg.DefaultRetentionDays
	}

	record := artifactRecord{
		Name:      name,
		Size:      size,
		SHA256:    digest,
		CreatedAt: now,
	}
	if retentionDays > 0 {
		record.ExpiresAt = now.Add(time.Duration(retentionDays) * 24 * time.Hour)
	}

	// Write the metadata file.
	metaData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling artifact metadata: %w", err)
	}

	if err := os.WriteFile(m.metaPath(jobID, name), metaData, 0o644); err != nil {
		return fmt.Errorf("writing artifact metadata: %w", err)
	}

	slog.Debug("uploaded artifact", "job_id", jobID, "name", name, "size", size, "sha256", digest)
	return nil
}

// Download retrieves a previously uploaded artifact. The returned ReadCloser
// provides the decompressed artifact content. The caller is responsible for
// closing it. Returns an error if the artifact does not exist.
func (m *Manager) Download(ctx context.Context, jobID string, name string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, err := os.Open(m.dataPath(jobID, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact %q for job %q not found: %w", name, jobID, err)
		}
		return nil, fmt.Errorf("opening artifact data file: %w", err)
	}

	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}

	// Return a reader that closes both the gzip reader and the underlying file.
	return &gzipFileReadCloser{gz: gr, f: f}, nil
}

// gzipFileReadCloser wraps a gzip.Reader and the underlying os.File so that
// closing it releases both resources.
type gzipFileReadCloser struct {
	gz *gzip.Reader
	f  *os.File
}

// Read implements io.Reader.
func (g *gzipFileReadCloser) Read(p []byte) (int, error) {
	return g.gz.Read(p)
}

// Close closes the gzip reader and the underlying file.
func (g *gzipFileReadCloser) Close() error {
	gzErr := g.gz.Close()
	fErr := g.f.Close()
	if gzErr != nil {
		return gzErr
	}
	return fErr
}

// List returns metadata for all artifacts associated with the given job.
func (m *Manager) List(ctx context.Context, jobID string) ([]api.ArtifactMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := m.jobDir(jobID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading job artifact directory: %w", err)
	}

	var artifacts []api.ArtifactMeta
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			slog.Warn("failed to read artifact metadata", "file", entry.Name(), "error", err)
			continue
		}

		var record artifactRecord
		if err := json.Unmarshal(data, &record); err != nil {
			slog.Warn("failed to parse artifact metadata", "file", entry.Name(), "error", err)
			continue
		}

		artifacts = append(artifacts, api.ArtifactMeta{
			Name:      record.Name,
			Size:      record.Size,
			SHA256:    record.SHA256,
			CreatedAt: record.CreatedAt,
			ExpiresAt: record.ExpiresAt,
		})
	}

	return artifacts, nil
}

// Delete removes a single artifact. It returns nil if the artifact does not exist.
func (m *Manager) Delete(ctx context.Context, jobID string, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove the data file.
	if err := os.Remove(m.dataPath(jobID, name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing artifact data file: %w", err)
	}

	// Remove the metadata file.
	if err := os.Remove(m.metaPath(jobID, name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing artifact metadata file: %w", err)
	}

	// Try to remove the job directory if it is now empty.
	_ = os.Remove(m.jobDir(jobID))

	return nil
}

// Verify that Manager implements ArtifactStore at compile time.
var _ ArtifactStore = (*Manager)(nil)

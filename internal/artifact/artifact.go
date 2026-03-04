// Package artifact defines the ArtifactStore interface for uploading,
// downloading, and managing build artifacts produced during workflow jobs.
package artifact

import (
	"context"
	"io"

	"github.com/nficano/github-runner/pkg/api"
)

// ArtifactStore abstracts the storage backend used for workflow artifacts.
// Every implementation must be safe for concurrent use.
type ArtifactStore interface {
	// Upload stores an artifact under the given job and name, reading
	// content from r. If an artifact with the same name already exists for
	// the job it is replaced. UploadOptions control retention and
	// compression behaviour.
	Upload(ctx context.Context, jobID string, name string, r io.Reader, opts api.UploadOptions) error

	// Download retrieves a previously uploaded artifact. The caller is
	// responsible for closing the returned ReadCloser. Returns an error if
	// the artifact does not exist.
	Download(ctx context.Context, jobID string, name string) (io.ReadCloser, error)

	// List returns metadata for all artifacts associated with the given job.
	List(ctx context.Context, jobID string) ([]api.ArtifactMeta, error)

	// Delete removes a single artifact. It returns nil if the artifact does
	// not exist.
	Delete(ctx context.Context, jobID string, name string) error
}

// Package secret defines the SecretProvider interface for resolving secrets
// during workflow execution. Implementations may back onto environment
// variables, encrypted files, or external secret managers such as Vault.
package secret

import (
	"context"

	"github.com/nficano/github-runner/pkg/api"
)

// SecretProvider retrieves named secrets for use during job execution.
// Implementations must be safe for concurrent use and should minimise the
// time that plaintext values reside in memory.
type SecretProvider interface {
	// GetSecret fetches a single secret by name. Callers must call
	// Secret.Zero when the returned value is no longer needed.
	GetSecret(ctx context.Context, name string) (api.Secret, error)

	// ListSecrets returns the names of all secrets available to the
	// current runner context. The values themselves are not returned.
	ListSecrets(ctx context.Context) ([]string, error)
}

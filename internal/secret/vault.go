package secret

import (
	"context"
	"fmt"

	"github.com/nficano/github-runner/pkg/api"
)

// VaultProvider is a scaffold implementation for HashiCorp Vault as a secret backend.
// A full implementation would use the Vault API client to retrieve secrets from
// a configured Vault server, supporting token-based and AppRole authentication,
// KV v1/v2 secret engines, and automatic token renewal.
type VaultProvider struct {
	addr      string
	token     string
	mountPath string
}

// VaultOptions configures the Vault secret provider.
type VaultOptions struct {
	// Address is the Vault server address (e.g., "https://vault.example.com:8200").
	Address string
	// Token is the Vault authentication token.
	Token string
	// MountPath is the KV secret engine mount path (default: "secret").
	MountPath string
}

// NewVaultProvider creates a new Vault-backed secret provider.
// This is a scaffold — a full implementation would initialize the Vault API client.
func NewVaultProvider(opts VaultOptions) *VaultProvider {
	mount := opts.MountPath
	if mount == "" {
		mount = "secret"
	}
	return &VaultProvider{
		addr:      opts.Address,
		token:     opts.Token,
		mountPath: mount,
	}
}

// GetSecret retrieves a secret from Vault.
// Scaffold: returns an error indicating this is not yet implemented.
func (v *VaultProvider) GetSecret(_ context.Context, name string) (api.Secret, error) {
	return api.Secret{}, fmt.Errorf("vault secret provider not implemented: cannot retrieve %q from %s", name, v.addr)
}

// ListSecrets lists available secrets in Vault.
// Scaffold: returns an error indicating this is not yet implemented.
func (v *VaultProvider) ListSecrets(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("vault secret provider not implemented: cannot list secrets at %s", v.addr)
}

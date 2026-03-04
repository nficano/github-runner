package secret

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/org/github-runner/pkg/api"
)

// MemoryStore is an in-memory secret provider that supports value zeroization
// after use. It is safe for concurrent access.
type MemoryStore struct {
	mu      sync.RWMutex
	secrets map[string]api.Secret
}

// NewMemoryStore creates a new in-memory secret store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		secrets: make(map[string]api.Secret),
	}
}

// Set adds or updates a secret in the store.
func (s *MemoryStore) Set(name string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Zero existing value if overwriting.
	if existing, ok := s.secrets[name]; ok {
		zeroBytes(existing.Value)
	}

	// Copy the value to avoid external mutation.
	v := make([]byte, len(value))
	copy(v, value)

	s.secrets[name] = api.Secret{
		Name:  name,
		Value: v,
	}
}

// GetSecret retrieves a secret by name.
func (s *MemoryStore) GetSecret(_ context.Context, name string) (api.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sec, ok := s.secrets[name]
	if !ok {
		return api.Secret{}, fmt.Errorf("secret %q not found", name)
	}

	// Return a copy to prevent external mutation.
	v := make([]byte, len(sec.Value))
	copy(v, sec.Value)
	return api.Secret{Name: sec.Name, Value: v}, nil
}

// ListSecrets returns the names of all stored secrets.
func (s *MemoryStore) ListSecrets(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	return names, nil
}

// ZeroAll overwrites all secret values in memory with zeros and clears the store.
// This should be called after job completion.
func (s *MemoryStore) ZeroAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sec := range s.secrets {
		zeroBytes(sec.Value)
	}
	s.secrets = make(map[string]api.Secret)
}

// zeroBytes overwrites a byte slice with zeros. Uses unsafe.Pointer to prevent
// compiler optimization from eliding the write.
//
//go:noinline
func zeroBytes(b []byte) {
	for i := range b {
		*(*byte)(unsafe.Pointer(&b[i])) = 0
	}
}

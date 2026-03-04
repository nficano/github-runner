package health

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Check is the interface that every health check must implement. A nil return
// means healthy; a non-nil error contains a human-readable failure reason.
type Check interface {
	Check(ctx context.Context) error
}

// CheckFunc adapts an ordinary function to the [Check] interface.
type CheckFunc func(ctx context.Context) error

// Check calls f.
func (f CheckFunc) Check(ctx context.Context) error { return f(ctx) }

// CheckRegistry holds named health checks and can execute them all.
type CheckRegistry struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewCheckRegistry returns an empty registry.
func NewCheckRegistry() *CheckRegistry {
	return &CheckRegistry{
		checks: make(map[string]Check),
	}
}

// Register adds a named check to the registry. If a check with the same name
// already exists it is replaced.
func (r *CheckRegistry) Register(name string, c Check) {
	r.mu.Lock()
	r.checks[name] = c
	r.mu.Unlock()
}

// RunAll executes every registered check and returns a map of check name to
// error (nil on success). Checks run concurrently and are bounded by ctx.
func (r *CheckRegistry) RunAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	names := make([]string, 0, len(r.checks))
	checkers := make([]Check, 0, len(r.checks))
	for name, c := range r.checks {
		names = append(names, name)
		checkers = append(checkers, c)
	}
	r.mu.RUnlock()

	results := make(map[string]error, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, c := range checkers {
		wg.Add(1)
		go func(name string, check Check) {
			defer wg.Done()
			err := check.Check(ctx)
			mu.Lock()
			results[name] = err
			mu.Unlock()
		}(names[i], c)
	}

	wg.Wait()
	return results
}

// ---------------------------------------------------------------------------
// Concrete health check implementations
// ---------------------------------------------------------------------------

// GitHubAPICheck verifies that the GitHub API is reachable by issuing a HEAD
// request to the API root URL.
type GitHubAPICheck struct {
	// APIURL is the base URL of the GitHub API (e.g.
	// "https://api.github.com"). For GitHub Enterprise Server this would be
	// the instance-specific URL.
	APIURL string

	// Client is the HTTP client used for the probe. If nil,
	// http.DefaultClient is used.
	Client *http.Client
}

// Check performs a HEAD request against the GitHub API root.
func (c *GitHubAPICheck) Check(ctx context.Context) error {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.APIURL, nil)
	if err != nil {
		return fmt.Errorf("github api check: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github api check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("github api check: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// DiskSpaceCheck verifies that the filesystem containing the given path has at
// least ThresholdBytes of free space.
type DiskSpaceCheck struct {
	// Path is the filesystem path to check (e.g. the runner's work
	// directory). The check uses statfs on the filesystem containing Path.
	Path string

	// ThresholdBytes is the minimum number of free bytes required for the
	// check to pass.
	ThresholdBytes uint64
}

// Check inspects available disk space on the filesystem containing c.Path.
func (c *DiskSpaceCheck) Check(ctx context.Context) error {
	var stat unix.Statfs_t
	if err := unix.Statfs(c.Path, &stat); err != nil {
		return fmt.Errorf("disk space check: statfs %s: %w", c.Path, err)
	}

	// Available blocks * block size = available bytes for unprivileged users.
	avail := uint64(stat.Bavail) * uint64(stat.Bsize)
	if avail < c.ThresholdBytes {
		return fmt.Errorf("disk space check: %d bytes available, need at least %d", avail, c.ThresholdBytes)
	}
	return nil
}

// ExecutorCheck verifies that the executor backend is operational. It runs the
// configured probe command (e.g. "docker info") and considers the backend
// healthy if the command exits 0 within the context deadline.
type ExecutorCheck struct {
	// Name is the human-readable name of the executor (e.g. "docker").
	Name string

	// ProbeCommand is the command used to verify that the executor backend
	// is available. For Docker this would be []string{"docker", "info"}.
	ProbeCommand []string
}

// Check runs the probe command and returns an error if it fails.
func (c *ExecutorCheck) Check(ctx context.Context) error {
	if len(c.ProbeCommand) == 0 {
		return fmt.Errorf("executor check %s: no probe command configured", c.Name)
	}

	cmd := exec.CommandContext(ctx, c.ProbeCommand[0], c.ProbeCommand[1:]...) //nolint:gosec // probe command is operator-configured
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("executor check %s: %w: %s", c.Name, err, string(output))
	}
	return nil
}

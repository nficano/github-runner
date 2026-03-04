// Package executor defines the Executor interface that all job execution
// backends must implement, along with a factory for instantiating them by name.
package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/nficano/github-runner/pkg/api"
)

// ExecutorType identifies a supported executor backend.
type ExecutorType string

const (
	// Shell executes steps directly in the host shell.
	Shell ExecutorType = "shell"
	// Docker executes steps inside Docker containers.
	Docker ExecutorType = "docker"
	// Kubernetes executes steps as Kubernetes pods.
	Kubernetes ExecutorType = "kubernetes"
	// Firecracker executes steps inside Firecracker micro-VMs.
	Firecracker ExecutorType = "firecracker"
)

// Executor defines the contract that every job execution backend must satisfy.
// Implementations are responsible for preparing the execution environment,
// running individual steps, and tearing down resources when the job completes.
type Executor interface {
	// Prepare initialises the execution environment for the given job.
	// It is called once before any steps are run and should set up
	// workspaces, pull images, or provision VMs as needed.
	Prepare(ctx context.Context, job *api.Job) error

	// Run executes a single step and returns its result. The caller is
	// responsible for invoking steps in order and respecting step conditions.
	Run(ctx context.Context, step *api.Step) (*api.StepResult, error)

	// Cleanup releases all resources associated with the current job.
	// It must be safe to call even if Prepare was never called or failed.
	Cleanup(ctx context.Context) error

	// Info returns metadata about this executor, including its name,
	// version, and supported feature set.
	Info() api.ExecutorInfo
}

// FactoryFunc is a constructor that creates an Executor from an opaque
// configuration value. The concrete type of cfg depends on the executor.
type FactoryFunc func(cfg interface{}) (Executor, error)

// registry holds the registered executor factories. Access is guarded by mu.
var (
	mu       sync.RWMutex
	registry = make(map[ExecutorType]FactoryFunc)
)

// Register makes a factory function available under the given executor type
// name. It is safe to call from multiple goroutines. Registering the same
// type twice overwrites the previous factory.
func Register(t ExecutorType, fn FactoryFunc) {
	mu.Lock()
	defer mu.Unlock()
	registry[t] = fn
}

// New creates an Executor of the specified type, passing cfg to its factory.
// Returns an error if no factory has been registered for the given name.
func New(name string, cfg interface{}) (Executor, error) {
	t := ExecutorType(name)

	mu.RLock()
	fn, ok := registry[t]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown executor type %q: %w", name, ErrUnknownExecutor)
	}
	exec, err := fn(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating executor %q: %w", name, err)
	}
	return exec, nil
}

// ErrUnknownExecutor is returned by New when the requested executor type has
// not been registered.
var ErrUnknownExecutor = fmt.Errorf("executor not registered")

// Package runner implements the core runner engine that manages worker pools,
// polls for jobs, and coordinates job execution.
package runner

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/nficano/github-runner/pkg/api"
)

// JobState represents the current state in the job lifecycle state machine.
type JobState int

const (
	// StateQueued is the initial state when a job is received from GitHub.
	StateQueued JobState = iota
	// StateClaimed means the runner has acquired the job.
	StateClaimed
	// StatePreparing means the executor is setting up the environment.
	StatePreparing
	// StateRunning means steps are being executed.
	StateRunning
	// StatePostExec means post-job tasks (artifacts, cache, hooks) are running.
	StatePostExec
	// StateCompleted means the job finished successfully.
	StateCompleted
	// StateFailed means the job finished with a failure.
	StateFailed
	// StateCancelled means the job was cancelled.
	StateCancelled
	// StateCleanup means the executor is tearing down resources.
	StateCleanup
)

// String returns the human-readable name of the job state.
func (s JobState) String() string {
	switch s {
	case StateQueued:
		return "queued"
	case StateClaimed:
		return "claimed"
	case StatePreparing:
		return "preparing"
	case StateRunning:
		return "running"
	case StatePostExec:
		return "post_exec"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCancelled:
		return "cancelled"
	case StateCleanup:
		return "cleanup"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// ToJobStatus converts a JobState to the corresponding api.JobStatus.
func (s JobState) ToJobStatus() api.JobStatus {
	switch s {
	case StateQueued:
		return api.JobQueued
	case StateClaimed, StatePreparing, StateRunning, StatePostExec, StateCleanup:
		return api.JobInProgress
	case StateCompleted:
		return api.JobCompleted
	case StateFailed:
		return api.JobFailed
	case StateCancelled:
		return api.JobCancelled
	default:
		return api.JobFailed
	}
}

// validTransitions defines the allowed state transitions.
var validTransitions = map[JobState][]JobState{
	StateQueued:    {StateClaimed, StateCancelled},
	StateClaimed:   {StatePreparing, StateFailed, StateCancelled},
	StatePreparing: {StateRunning, StateFailed, StateCancelled},
	StateRunning:   {StatePostExec, StateFailed, StateCancelled},
	StatePostExec:  {StateCompleted, StateFailed, StateCancelled},
	StateCompleted: {StateCleanup},
	StateFailed:    {StateCleanup},
	StateCancelled: {StateCleanup},
}

// Lifecycle tracks the state machine for a single job's execution.
// All state transitions are validated and logged. Thread-safe.
type Lifecycle struct {
	mu      sync.RWMutex
	state   JobState
	jobID   int64
	logger  *slog.Logger
	onTransition func(from, to JobState)
}

// NewLifecycle creates a new lifecycle tracker for the given job.
func NewLifecycle(jobID int64, logger *slog.Logger) *Lifecycle {
	return &Lifecycle{
		state:  StateQueued,
		jobID:  jobID,
		logger: logger,
	}
}

// OnTransition sets a callback that fires on every valid state transition.
func (l *Lifecycle) OnTransition(fn func(from, to JobState)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onTransition = fn
}

// State returns the current job state. Thread-safe.
func (l *Lifecycle) State() JobState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// Transition attempts to move the job to the given state. Returns an error
// if the transition is not valid from the current state.
func (l *Lifecycle) Transition(to JobState) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	from := l.state
	allowed := validTransitions[from]
	for _, a := range allowed {
		if a == to {
			l.state = to
			l.logger.Info("job state transition",
				slog.Int64("job_id", l.jobID),
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
			if l.onTransition != nil {
				l.onTransition(from, to)
			}
			return nil
		}
	}

	return fmt.Errorf("invalid job state transition from %s to %s for job %d",
		from.String(), to.String(), l.jobID)
}

// IsTerminal returns true if the current state is a terminal state.
func (l *Lifecycle) IsTerminal() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	switch l.state {
	case StateCleanup:
		return true
	default:
		return false
	}
}

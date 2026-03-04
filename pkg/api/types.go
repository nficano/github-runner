// Package api defines the shared types used across the github-runner plugin
// interface. All types in this package are safe for concurrent read access.
// Mutable state (e.g., Secret values) must be protected by the caller.
package api

import (
	"time"
)

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// JobStatus represents the lifecycle state of a job.
type JobStatus string

const (
	// JobQueued indicates the job is waiting to be picked up by a runner.
	JobQueued JobStatus = "queued"
	// JobInProgress indicates the job is currently executing.
	JobInProgress JobStatus = "in_progress"
	// JobCompleted indicates the job finished successfully.
	JobCompleted JobStatus = "completed"
	// JobFailed indicates the job terminated with a failure.
	JobFailed JobStatus = "failed"
	// JobCancelled indicates the job was cancelled before completion.
	JobCancelled JobStatus = "cancelled"
)

// StepStatus represents the execution state of a single step.
type StepStatus string

const (
	// StepPending indicates the step has not started yet.
	StepPending StepStatus = "pending"
	// StepRunning indicates the step is currently executing.
	StepRunning StepStatus = "running"
	// StepCompleted indicates the step has finished execution.
	StepCompleted StepStatus = "completed"
	// StepFailed indicates the step terminated with a failure.
	StepFailed StepStatus = "failed"
	// StepSkipped indicates the step was skipped due to a condition.
	StepSkipped StepStatus = "skipped"
)

// StepConclusion represents the final outcome of a completed step.
type StepConclusion string

const (
	// ConclusionSuccess indicates the step succeeded.
	ConclusionSuccess StepConclusion = "success"
	// ConclusionFailure indicates the step failed.
	ConclusionFailure StepConclusion = "failure"
	// ConclusionSkipped indicates the step was skipped.
	ConclusionSkipped StepConclusion = "skipped"
	// ConclusionCancelled indicates the step was cancelled.
	ConclusionCancelled StepConclusion = "cancelled"
)

// HookEvent identifies the point in the job lifecycle at which a hook fires.
type HookEvent string

const (
	// HookPreJob fires before any step in the job executes.
	HookPreJob HookEvent = "pre_job"
	// HookPostJob fires after all steps in the job have completed.
	HookPostJob HookEvent = "post_job"
	// HookPreStep fires before an individual step executes.
	HookPreStep HookEvent = "pre_step"
	// HookPostStep fires after an individual step completes.
	HookPostStep HookEvent = "post_step"
)

// ---------------------------------------------------------------------------
// Executor metadata
// ---------------------------------------------------------------------------

// ExecutorInfo describes the capabilities and identity of an executor backend.
type ExecutorInfo struct {
	// Name is the human-readable name of the executor (e.g., "docker", "shell").
	Name string `json:"name"`
	// Version is the semantic version of the executor implementation.
	Version string `json:"version"`
	// Features lists optional capabilities the executor supports
	// (e.g., "container-networking", "gpu").
	Features []string `json:"features,omitempty"`
}

// ---------------------------------------------------------------------------
// Job & Step
// ---------------------------------------------------------------------------

// Job represents a single GitHub Actions workflow job to be executed.
type Job struct {
	// ID is the unique identifier for this job assigned by GitHub.
	ID int64 `json:"id"`
	// RunID is the identifier of the workflow run this job belongs to.
	RunID int64 `json:"run_id"`
	// Name is the display name of the job as defined in the workflow YAML.
	Name string `json:"name"`
	// WorkflowName is the name of the parent workflow.
	WorkflowName string `json:"workflow_name"`
	// Labels are the runner labels required to pick up this job.
	Labels []string `json:"labels,omitempty"`
	// Steps is the ordered list of steps to execute.
	Steps []Step `json:"steps,omitempty"`
	// Secrets contains the secret name-to-value mappings available to this job.
	Secrets map[string]string `json:"secrets,omitempty"`
	// Env holds the environment variables scoped to this job.
	Env map[string]string `json:"env,omitempty"`
	// Workspace is the filesystem path where the job should execute.
	Workspace string `json:"workspace"`
	// Repository is the full owner/name of the repository (e.g., "nficano/github-runner").
	Repository string `json:"repository"`
	// Ref is the git ref that triggered the workflow (e.g., "refs/heads/main").
	Ref string `json:"ref"`
	// SHA is the commit SHA that triggered the workflow.
	SHA string `json:"sha"`
	// Status is the current lifecycle state of the job.
	Status JobStatus `json:"status"`
	// Timeout is the maximum duration the job is allowed to run.
	Timeout time.Duration `json:"timeout"`
	// CreatedAt is the timestamp when the job was created.
	CreatedAt time.Time `json:"created_at"`
	// StartedAt is the timestamp when the job began executing.
	// Zero value indicates the job has not started.
	StartedAt time.Time `json:"started_at,omitempty"`
}

// Step represents a single step within a job.
type Step struct {
	// ID is the unique identifier for this step within the job.
	ID string `json:"id"`
	// Name is the display name of the step.
	Name string `json:"name"`
	// Run is the shell command to execute (mutually exclusive with Uses).
	Run string `json:"run,omitempty"`
	// Uses identifies the action to run (e.g., "actions/checkout@v4").
	Uses string `json:"uses,omitempty"`
	// With contains input parameters passed to the action.
	With map[string]string `json:"with,omitempty"`
	// Env holds the environment variables scoped to this step.
	Env map[string]string `json:"env,omitempty"`
	// If is a conditional expression that determines whether the step runs.
	If string `json:"if,omitempty"`
	// TimeoutMinutes is the maximum number of minutes this step may run.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
	// ContinueOnError, when true, allows the job to continue if this step fails.
	ContinueOnError bool `json:"continue_on_error,omitempty"`
	// Shell specifies the shell used to run the command (e.g., "bash", "pwsh").
	Shell string `json:"shell,omitempty"`
	// WorkingDirectory overrides the default working directory for this step.
	WorkingDirectory string `json:"working_directory,omitempty"`
}

// StepResult captures the outcome of executing a single step.
type StepResult struct {
	// StepID links this result back to the step definition.
	StepID string `json:"step_id"`
	// Status is the final execution state.
	Status StepStatus `json:"status"`
	// Conclusion is the outcome determination (success, failure, etc.).
	Conclusion StepConclusion `json:"conclusion"`
	// StartedAt is the timestamp when the step began executing.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is the timestamp when the step finished.
	CompletedAt time.Time `json:"completed_at"`
	// ExitCode is the process exit code. -1 typically indicates the process
	// was killed or never started.
	ExitCode int `json:"exit_code"`
	// Output contains any key-value outputs produced by the step.
	Output map[string]string `json:"output,omitempty"`
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

// CacheOptions configures the behavior of a cache put operation.
type CacheOptions struct {
	// Key is the primary lookup key for the cache entry.
	Key string `json:"key"`
	// RestoreKeys is an ordered list of fallback key prefixes used when the
	// primary key does not match.
	RestoreKeys []string `json:"restore_keys,omitempty"`
	// Paths lists the filesystem paths to include in the cache entry.
	Paths []string `json:"paths,omitempty"`
	// Scope limits cache visibility (e.g., to a particular branch).
	Scope string `json:"scope,omitempty"`
	// TTL is the time-to-live for the cache entry. Zero means no expiry.
	TTL time.Duration `json:"ttl,omitempty"`
}

// CacheStats reports aggregate statistics for the cache backend.
type CacheStats struct {
	// Entries is the total number of cached items.
	Entries int64 `json:"entries"`
	// TotalSize is the total size of all cached items in bytes.
	TotalSize int64 `json:"total_size"`
	// HitCount is the cumulative number of cache hits.
	HitCount int64 `json:"hit_count"`
	// MissCount is the cumulative number of cache misses.
	MissCount int64 `json:"miss_count"`
	// EvictionCount is the cumulative number of evicted entries.
	EvictionCount int64 `json:"eviction_count"`
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

// UploadOptions configures artifact upload behavior.
type UploadOptions struct {
	// RetentionDays is the number of days to retain the artifact.
	// Zero means use the repository or organization default.
	RetentionDays int `json:"retention_days,omitempty"`
	// CompressionLevel controls the compression level (0-9).
	// Zero means default compression.
	CompressionLevel int `json:"compression_level,omitempty"`
}

// ArtifactMeta contains metadata about a stored artifact.
type ArtifactMeta struct {
	// Name is the display name of the artifact.
	Name string `json:"name"`
	// Size is the uncompressed size of the artifact in bytes.
	Size int64 `json:"size"`
	// SHA256 is the hex-encoded SHA-256 digest of the artifact contents.
	SHA256 string `json:"sha256"`
	// CreatedAt is the timestamp when the artifact was uploaded.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is the timestamp when the artifact will be automatically deleted.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ---------------------------------------------------------------------------
// Secrets
// ---------------------------------------------------------------------------

// Secret holds a named secret value. The underlying byte slice should be
// zeroed after use by calling Zero.
type Secret struct {
	// Name is the identifier of the secret.
	Name string `json:"name"`
	// Value is the raw secret material. Callers must call Zero when the
	// secret is no longer needed.
	Value []byte `json:"-"`
}

// Zero overwrites the secret value with zeroes, making the sensitive material
// unrecoverable from memory. This should be called as soon as the secret is
// no longer needed.
func (s *Secret) Zero() {
	for i := range s.Value {
		s.Value[i] = 0
	}
}

// ---------------------------------------------------------------------------
// Registration & configuration
// ---------------------------------------------------------------------------

// RegisterOptions contains the parameters needed to register a new runner
// with a GitHub instance.
type RegisterOptions struct {
	// URL is the base URL of the GitHub instance (e.g., "https://github.com").
	URL string `json:"url"`
	// Token is the one-time registration token.
	Token string `json:"token"`
	// Name is the desired display name for the runner.
	Name string `json:"name"`
	// Labels are the custom labels to assign to the runner.
	Labels []string `json:"labels,omitempty"`
	// Executor is the name of the executor backend to use.
	Executor string `json:"executor"`
	// WorkDir is the root directory where job workspaces will be created.
	WorkDir string `json:"work_dir"`
	// Ephemeral, when true, causes the runner to de-register itself after
	// completing a single job.
	Ephemeral bool `json:"ephemeral,omitempty"`
}

// RunnerConfig holds the persisted configuration for a registered runner.
type RunnerConfig struct {
	// ID is the unique identifier assigned by GitHub upon registration.
	ID int64 `json:"id"`
	// Name is the display name of the runner.
	Name string `json:"name"`
	// Token is the long-lived authentication token for the runner.
	Token string `json:"token"`
	// URL is the base URL of the GitHub instance.
	URL string `json:"url"`
	// Labels are the labels assigned to the runner.
	Labels []string `json:"labels,omitempty"`
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

// HookResult captures the outcome of a hook execution.
type HookResult struct {
	// Output is the combined stdout/stderr output from the hook.
	Output string `json:"output,omitempty"`
	// ExitCode is the exit code of the hook process.
	ExitCode int `json:"exit_code"`
	// Duration is the wall-clock time the hook took to execute.
	Duration time.Duration `json:"duration"`
}

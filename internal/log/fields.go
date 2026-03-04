// Package log provides structured logging infrastructure for the GitHub Actions
// runner. It defines standard field names, a secret-masking handler, and helpers
// for configuring slog-based loggers.
package log

// Standard structured-log field names used throughout the runner codebase.
// Using constants avoids typos and makes it easy to grep for every place a
// particular field is emitted.
const (
	// FieldJobID is the unique identifier of the job being executed.
	FieldJobID = "job_id"

	// FieldRunnerName is the human-readable name of the runner instance.
	FieldRunnerName = "runner_name"

	// FieldPoolName is the name of the runner pool this runner belongs to.
	FieldPoolName = "pool_name"

	// FieldRepo is the owner/repo slug (e.g. "org/repo").
	FieldRepo = "repository"

	// FieldWorkflow is the name or path of the workflow file.
	FieldWorkflow = "workflow"

	// FieldStep is the name or index of the current workflow step.
	FieldStep = "step"

	// FieldExecutor is the executor backend in use (docker, shell, etc.).
	FieldExecutor = "executor"

	// FieldDuration is the elapsed wall-clock time in milliseconds.
	FieldDuration = "duration_ms"

	// FieldError is the error string attached to a log record.
	FieldError = "error"

	// FieldComponent is the subsystem that produced the log record.
	FieldComponent = "component"
)

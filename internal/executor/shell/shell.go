package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/org/github-runner/internal/executor"
	"github.com/org/github-runner/pkg/api"
)

const (
	// defaultShell is used when no shell is explicitly configured.
	defaultShell = "bash"
	// defaultTimeout is the per-step timeout when none is specified.
	defaultTimeout = 60 * time.Minute
	// version is the semantic version of the shell executor.
	version = "0.1.0"
)

// ShellConfig configures the shell executor.
type ShellConfig struct {
	// WorkDir is the root directory under which per-job workspace directories
	// are created. Each job gets its own subdirectory.
	WorkDir string

	// Shell is the shell binary used to execute run commands (e.g., "bash", "sh").
	// Defaults to "bash" if empty.
	Shell string

	// EnvAllowlist is an optional list of host environment variable names to
	// propagate into the step process. When non-empty, only these variables
	// (plus job/step-scoped env) are visible.
	EnvAllowlist []string

	// EnvDenylist is a list of host environment variable names that must never
	// be propagated into the step process. Denylist entries take precedence
	// over allowlist entries.
	EnvDenylist []string

	// Stdout is the writer for standard output from step commands.
	// If nil, output is discarded.
	Stdout io.Writer

	// Stderr is the writer for standard error from step commands.
	// If nil, output is discarded.
	Stderr io.Writer
}

// ShellExecutor runs job steps as host processes using os/exec.
type ShellExecutor struct {
	cfg       ShellConfig
	logger    *slog.Logger
	job       *api.Job
	workspace string
}

// New creates a new ShellExecutor with the given configuration.
func New(cfg ShellConfig) (*ShellExecutor, error) {
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("shell executor: work directory must not be empty")
	}
	if cfg.Shell == "" {
		cfg.Shell = defaultShell
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}

	return &ShellExecutor{
		cfg:    cfg,
		logger: slog.Default().With("executor", "shell"),
	}, nil
}

// factory creates a ShellExecutor from an opaque config value.
func factory(cfg interface{}) (executor.Executor, error) {
	c, ok := cfg.(ShellConfig)
	if !ok {
		return nil, fmt.Errorf("shell executor: expected ShellConfig, got %T", cfg)
	}
	return New(c)
}

// Register registers the shell executor factory with the executor registry.
// This should be called during application startup.
func Register() {
	executor.Register(executor.Shell, factory)
}

// Info returns metadata about the shell executor.
func (e *ShellExecutor) Info() api.ExecutorInfo {
	return api.ExecutorInfo{
		Name:     "shell",
		Version:  version,
		Features: []string{"host-execution", "env-filtering"},
	}
}

// Prepare creates the workspace directory for the given job and stores the
// job reference for use during step execution.
func (e *ShellExecutor) Prepare(ctx context.Context, job *api.Job) error {
	e.job = job
	e.workspace = filepath.Join(e.cfg.WorkDir, fmt.Sprintf("job-%d", job.ID))

	e.logger.InfoContext(ctx, "preparing workspace",
		"job_id", job.ID,
		"workspace", e.workspace,
	)

	if err := os.MkdirAll(e.workspace, 0o755); err != nil {
		return fmt.Errorf("creating workspace %s: %w", e.workspace, err)
	}

	return nil
}

// Run executes a single step using the configured shell. It captures stdout
// and stderr in real-time via the configured writers, enforces timeouts, and
// returns a StepResult describing the outcome.
func (e *ShellExecutor) Run(ctx context.Context, step *api.Step) (*api.StepResult, error) {
	if e.job == nil {
		return nil, fmt.Errorf("shell executor: Run called before Prepare")
	}

	result := &api.StepResult{
		StepID:    step.ID,
		Status:    api.StepRunning,
		StartedAt: time.Now(),
		ExitCode:  -1,
	}

	// Determine the per-step timeout.
	timeout := defaultTimeout
	if step.TimeoutMinutes > 0 {
		timeout = time.Duration(step.TimeoutMinutes) * time.Minute
	}

	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Determine the shell to use.
	shell := e.cfg.Shell
	if step.Shell != "" {
		shell = step.Shell
	}

	// Determine working directory.
	workDir := e.workspace
	if step.WorkingDirectory != "" {
		workDir = filepath.Join(e.workspace, step.WorkingDirectory)
	}

	e.logger.InfoContext(ctx, "running step",
		"step_id", step.ID,
		"step_name", step.Name,
		"shell", shell,
		"work_dir", workDir,
		"timeout", timeout,
	)

	// Build the command.
	cmd := exec.CommandContext(stepCtx, shell, "-e", "-c", step.Run)
	cmd.Dir = workDir
	cmd.Stdout = e.cfg.Stdout
	cmd.Stderr = e.cfg.Stderr

	// Build the environment.
	env, err := e.buildEnv(step)
	if err != nil {
		e.logger.WarnContext(ctx, "environment build produced warnings", "error", err)
	}
	cmd.Env = env

	// Execute the command.
	if err := cmd.Run(); err != nil {
		result.CompletedAt = time.Now()

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Status = api.StepCompleted
			result.Conclusion = api.ConclusionFailure
			e.logger.InfoContext(ctx, "step failed",
				"step_id", step.ID,
				"exit_code", result.ExitCode,
			)
			return result, nil
		}

		// Context-related errors (timeout/cancellation).
		if stepCtx.Err() != nil {
			result.Status = api.StepCompleted
			result.Conclusion = api.ConclusionFailure
			e.logger.WarnContext(ctx, "step timed out or was cancelled",
				"step_id", step.ID,
				"ctx_err", stepCtx.Err(),
			)
			return result, nil
		}

		return nil, fmt.Errorf("executing step %s: %w", step.ID, err)
	}

	result.CompletedAt = time.Now()
	result.ExitCode = 0
	result.Status = api.StepCompleted
	result.Conclusion = api.ConclusionSuccess

	e.logger.InfoContext(ctx, "step completed",
		"step_id", step.ID,
		"duration", result.CompletedAt.Sub(result.StartedAt),
	)

	return result, nil
}

// Cleanup removes the workspace directory. It is safe to call even if Prepare
// was never called.
func (e *ShellExecutor) Cleanup(ctx context.Context) error {
	if e.workspace == "" {
		return nil
	}

	e.logger.InfoContext(ctx, "cleaning up workspace", "workspace", e.workspace)

	if err := os.RemoveAll(e.workspace); err != nil {
		return fmt.Errorf("removing workspace %s: %w", e.workspace, err)
	}

	e.workspace = ""
	e.job = nil
	return nil
}

// buildEnv constructs the environment variable slice for a step command by
// filtering the host environment and merging in job- and step-level variables.
func (e *ShellExecutor) buildEnv(step *api.Step) ([]string, error) {
	// Start with filtered host environment.
	hostEnv := FilterEnv(os.Environ(), e.cfg.EnvAllowlist, e.cfg.EnvDenylist)

	// Build the override map from job env + step env (step wins).
	overrides := make(map[string]string)
	for k, v := range e.job.Env {
		overrides[k] = v
	}
	for k, v := range step.Env {
		overrides[k] = v
	}

	// Add runner-specific variables.
	overrides["GITHUB_WORKSPACE"] = e.workspace
	overrides["RUNNER_WORKSPACE"] = e.workspace
	if e.job.Repository != "" {
		overrides["GITHUB_REPOSITORY"] = e.job.Repository
	}
	if e.job.Ref != "" {
		overrides["GITHUB_REF"] = e.job.Ref
	}
	if e.job.SHA != "" {
		overrides["GITHUB_SHA"] = e.job.SHA
	}

	// Parse host env into a map.
	hostMap := make(map[string]string, len(hostEnv))
	for _, entry := range hostEnv {
		key, value, ok := parseEnvEntry(entry)
		if ok {
			hostMap[key] = value
		}
	}

	// Merge and return.
	return MergeEnv(hostMap, overrides)
}

// Output returns the combined captured output from the last step. This is
// primarily useful for testing; production use should rely on the real-time
// io.Writer streams.
func (e *ShellExecutor) Output() string {
	if buf, ok := e.cfg.Stdout.(*bytes.Buffer); ok {
		return buf.String()
	}
	return ""
}

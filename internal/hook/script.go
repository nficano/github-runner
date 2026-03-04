package hook

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/nficano/github-runner/pkg/api"
)

// ScriptHook executes a shell script or binary as a hook.
type ScriptHook struct {
	// Path is the path to the script or binary to execute.
	Path string
	// Timeout is the maximum duration the hook may run.
	Timeout time.Duration
	// Env contains additional environment variables for the hook process.
	Env map[string]string
	logger *slog.Logger
}

// NewScriptHook creates a new script-based hook.
func NewScriptHook(path string, timeout time.Duration, logger *slog.Logger) *ScriptHook {
	return &ScriptHook{
		Path:    path,
		Timeout: timeout,
		Env:     make(map[string]string),
		logger:  logger,
	}
}

// Execute runs the hook script, passing job metadata as environment variables.
func (h *ScriptHook) Execute(ctx context.Context, event api.HookEvent, job *api.Job) error {
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, h.Path)

	// Set environment variables with job context.
	env := []string{
		fmt.Sprintf("GITHUB_RUNNER_HOOK_EVENT=%s", event),
		fmt.Sprintf("GITHUB_JOB_ID=%d", job.ID),
		fmt.Sprintf("GITHUB_REPOSITORY=%s", job.Repository),
		fmt.Sprintf("GITHUB_WORKFLOW=%s", job.WorkflowName),
		fmt.Sprintf("GITHUB_REF=%s", job.Ref),
		fmt.Sprintf("GITHUB_SHA=%s", job.SHA),
		fmt.Sprintf("GITHUB_WORKSPACE=%s", job.Workspace),
	}
	for k, v := range h.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	h.logger.InfoContext(ctx, "hook executed",
		slog.String("path", h.Path),
		slog.String("event", string(event)),
		slog.Int64("job_id", job.ID),
		slog.Duration("duration", duration),
	)

	if err != nil {
		return fmt.Errorf("hook %q failed (event=%s, duration=%s): %w\nstderr: %s",
			h.Path, event, duration, err, stderr.String())
	}

	return nil
}

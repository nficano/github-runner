package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/org/github-runner/internal/executor"
	"github.com/org/github-runner/pkg/api"
)

// StepRunner executes individual steps within a job, handling timeouts,
// conditional execution, and result collection.
type StepRunner struct {
	exec   executor.Executor
	jobCtx *Context
	logger *slog.Logger
}

// NewStepRunner creates a new step runner.
func NewStepRunner(exec executor.Executor, jobCtx *Context, logger *slog.Logger) *StepRunner {
	return &StepRunner{
		exec:   exec,
		jobCtx: jobCtx,
		logger: logger,
	}
}

// RunStep executes a single step with timeout enforcement and result tracking.
func (sr *StepRunner) RunStep(ctx context.Context, job *api.Job, step *api.Step) (*api.StepResult, error) {
	if err := ValidateStep(step); err != nil {
		return &api.StepResult{
			StepID:      step.ID,
			Status:      api.StepFailed,
			Conclusion:  api.ConclusionFailure,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
			ExitCode:    -1,
		}, fmt.Errorf("step validation: %w", err)
	}

	// Evaluate conditional.
	if step.If != "" {
		// Scaffold: expression evaluation is not yet implemented.
		// A full implementation would evaluate GitHub Actions expressions
		// like "success()", "failure()", "always()", and custom expressions.
		sr.logger.DebugContext(ctx, "step condition evaluation not implemented, running step",
			slog.String("step", step.Name),
			slog.String("if", step.If),
		)
	}

	// Apply step timeout.
	timeout := StepTimeout(job, step)
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	sr.logger.InfoContext(ctx, "step started",
		slog.String("step", step.Name),
		slog.Duration("timeout", timeout),
	)

	result, err := sr.exec.Run(stepCtx, step)
	completedAt := time.Now()

	if result == nil {
		result = &api.StepResult{
			StepID:      step.ID,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		}
	} else {
		result.StartedAt = startedAt
		result.CompletedAt = completedAt
	}

	if err != nil {
		result.Status = api.StepFailed
		result.Conclusion = api.ConclusionFailure
		if stepCtx.Err() == context.DeadlineExceeded {
			sr.logger.WarnContext(ctx, "step timed out",
				slog.String("step", step.Name),
				slog.Duration("timeout", timeout),
			)
		}
		return result, fmt.Errorf("running step %q: %w", step.Name, err)
	}

	sr.logger.InfoContext(ctx, "step completed",
		slog.String("step", step.Name),
		slog.String("conclusion", string(result.Conclusion)),
		slog.Duration("duration", completedAt.Sub(startedAt)),
	)

	// Record result for expression evaluation in subsequent steps.
	sr.jobCtx.SetStepResult(step.ID, result)

	return result, nil
}

package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/nficano/github-runner/internal/executor"
	"github.com/nficano/github-runner/internal/github"
	"github.com/nficano/github-runner/internal/hook"
	"github.com/nficano/github-runner/internal/secret"
	"github.com/nficano/github-runner/pkg/api"
)

// Worker executes a single job. Each active job gets its own Worker goroutine.
// Workers are created by the Pool when a job is dispatched.
type Worker struct {
	id       int
	runnerID int64
	executor executor.Executor
	client   github.GitHubClient
	hooks    *hook.HookChain
	masker   *secret.Masker
	output   io.Writer
	logger   *slog.Logger
}

// WorkerConfig holds the dependencies needed to create a Worker.
type WorkerConfig struct {
	ID       int
	RunnerID int64
	Executor executor.Executor
	Client   github.GitHubClient
	Hooks    *hook.HookChain
	Masker   *secret.Masker
	Output   io.Writer
	Logger   *slog.Logger
}

// NewWorker creates a new Worker with the given dependencies.
func NewWorker(cfg WorkerConfig) *Worker {
	return &Worker{
		id:       cfg.ID,
		runnerID: cfg.RunnerID,
		executor: cfg.Executor,
		client:   cfg.Client,
		hooks:    cfg.Hooks,
		masker:   cfg.Masker,
		output:   cfg.Output,
		logger:   cfg.Logger,
	}
}

// Execute runs a job through its complete lifecycle. It manages state transitions,
// executor operations, heartbeat reporting, and cleanup. This method blocks until
// the job completes or the context is cancelled.
func (w *Worker) Execute(ctx context.Context, jobResp *github.JobResponse) error {
	job := jobResponseToJob(jobResp)
	lc := NewLifecycle(job.ID, w.logger)

	// Register secrets for masking.
	for _, v := range job.Secrets {
		w.masker.AddSecret(v)
	}

	// Always run cleanup and flush masker, even on error.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_ = lc.Transition(StateCleanup)
		if err := w.executor.Cleanup(cleanupCtx); err != nil {
			w.logger.ErrorContext(ctx, "executor cleanup failed",
				slog.Int64("job_id", job.ID),
				slog.String("error", err.Error()),
			)
		}
		_ = w.masker.Flush()
	}()

	// Start heartbeat reporter in background.
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	hb := NewHeartbeatReporter(w.client, w.runnerID, job.ID, w.logger)
	go hb.Run(heartbeatCtx)

	// Transition: Queued -> Claimed.
	if err := lc.Transition(StateClaimed); err != nil {
		return fmt.Errorf("worker %d: %w", w.id, err)
	}
	if err := w.client.ReportJobStatus(ctx, job.ID, api.JobInProgress); err != nil {
		w.logger.WarnContext(ctx, "failed to report job claimed",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
	}

	// Run pre-job hooks.
	if w.hooks != nil && w.hooks.Len() > 0 {
		if err := w.hooks.Execute(ctx, api.HookPreJob, job); err != nil {
			w.logger.WarnContext(ctx, "pre-job hook failed",
				slog.Int64("job_id", job.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Transition: Claimed -> Preparing.
	if err := lc.Transition(StatePreparing); err != nil {
		return fmt.Errorf("worker %d: %w", w.id, err)
	}

	// Prepare executor environment.
	if err := w.executor.Prepare(ctx, job); err != nil {
		_ = lc.Transition(StateFailed)
		_ = w.client.ReportJobStatus(ctx, job.ID, api.JobFailed)
		return fmt.Errorf("worker %d: executor prepare: %w", w.id, err)
	}

	// Transition: Preparing -> Running.
	if err := lc.Transition(StateRunning); err != nil {
		return fmt.Errorf("worker %d: %w", w.id, err)
	}

	// Execute steps sequentially.
	var jobFailed bool
	for i := range job.Steps {
		step := &job.Steps[i]

		// Check context cancellation before each step.
		if ctx.Err() != nil {
			_ = lc.Transition(StateCancelled)
			_ = w.client.ReportJobStatus(ctx, job.ID, api.JobCancelled)
			return fmt.Errorf("worker %d: job cancelled: %w", w.id, ctx.Err())
		}

		// Apply step timeout if configured.
		stepCtx := ctx
		if step.TimeoutMinutes > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutMinutes)*time.Minute)
			defer cancel()
		}

		result, err := w.executor.Run(stepCtx, step)
		if err != nil {
			w.logger.ErrorContext(ctx, "step execution failed",
				slog.Int64("job_id", job.ID),
				slog.String("step", step.Name),
				slog.String("error", err.Error()),
			)
			if !step.ContinueOnError {
				jobFailed = true
				break
			}
		}

		// Report step status to GitHub.
		if result != nil {
			if reportErr := w.client.ReportStepStatus(ctx, job.ID, result); reportErr != nil {
				w.logger.WarnContext(ctx, "failed to report step status",
					slog.Int64("job_id", job.ID),
					slog.String("step", step.Name),
					slog.String("error", reportErr.Error()),
				)
			}
			if result.Conclusion == api.ConclusionFailure && !step.ContinueOnError {
				jobFailed = true
				break
			}
		}
	}

	// Transition: Running -> PostExec.
	if err := lc.Transition(StatePostExec); err != nil {
		return fmt.Errorf("worker %d: %w", w.id, err)
	}

	// Run post-job hooks (even on failure).
	if w.hooks != nil && w.hooks.Len() > 0 {
		if err := w.hooks.Execute(ctx, api.HookPostJob, job); err != nil {
			w.logger.WarnContext(ctx, "post-job hook failed",
				slog.Int64("job_id", job.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Final status.
	if jobFailed {
		_ = lc.Transition(StateFailed)
		_ = w.client.ReportJobStatus(ctx, job.ID, api.JobFailed)
	} else {
		_ = lc.Transition(StateCompleted)
		_ = w.client.ReportJobStatus(ctx, job.ID, api.JobCompleted)
	}

	return nil
}

// jobResponseToJob converts a GitHub API job response to the internal job type.
func jobResponseToJob(resp *github.JobResponse) *api.Job {
	job := &api.Job{
		ID:           resp.ID,
		RunID:        resp.RunID,
		Name:         resp.Name,
		WorkflowName: resp.WorkflowName,
		Labels:       resp.Labels,
		Secrets:      resp.Secrets,
		Env:          resp.Env,
		Workspace:    resp.Workspace,
		Repository:   resp.Repository,
		Ref:          resp.Ref,
		SHA:          resp.SHA,
		Status:       api.JobQueued,
		CreatedAt:    resp.CreatedAt,
	}

	if resp.TimeoutMinutes > 0 {
		job.Timeout = time.Duration(resp.TimeoutMinutes) * time.Minute
	} else {
		job.Timeout = 6 * time.Hour // GitHub default.
	}

	for _, s := range resp.Steps {
		job.Steps = append(job.Steps, api.Step{
			ID:               s.ID,
			Name:             s.Name,
			Run:              s.Run,
			Uses:             s.Uses,
			With:             s.With,
			Env:              s.Env,
			If:               s.If,
			TimeoutMinutes:   s.TimeoutMinutes,
			ContinueOnError:  s.ContinueOnError,
			Shell:            s.Shell,
			WorkingDirectory: s.WorkingDirectory,
		})
	}

	return job
}

package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/org/github-runner/internal/config"
	"github.com/org/github-runner/internal/executor"
	"github.com/org/github-runner/internal/github"
	"github.com/org/github-runner/internal/hook"
	"github.com/org/github-runner/internal/secret"
)

// Pool manages a set of workers for a single [[runners]] configuration entry.
// It owns the poller and dispatches jobs to available workers via a buffered channel.
type Pool struct {
	name        string
	runnerID    int64
	cfg         config.RunnerConfig
	client      github.GitHubClient
	hooks       *hook.HookChain
	output      io.Writer
	logger      *slog.Logger

	// activeJobs tracks the number of currently executing jobs.
	// Synchronized via atomic operations for lock-free metrics reads.
	activeJobs atomic.Int64

	// jobs is the buffered channel connecting the poller to workers.
	// Buffer size equals the configured concurrency.
	jobs chan *github.JobResponse
}

// PoolConfig holds the dependencies needed to create a Pool.
type PoolConfig struct {
	RunnerID int64
	Config   config.RunnerConfig
	Client   github.GitHubClient
	Hooks    *hook.HookChain
	Output   io.Writer
	Logger   *slog.Logger
}

// NewPool creates a new worker pool for the given runner configuration.
func NewPool(cfg PoolConfig) *Pool {
	return &Pool{
		name:     cfg.Config.Name,
		runnerID: cfg.RunnerID,
		cfg:      cfg.Config,
		client:   cfg.Client,
		hooks:    cfg.Hooks,
		output:   cfg.Output,
		logger:   cfg.Logger.With(slog.String("pool", cfg.Config.Name)),
		jobs:     make(chan *github.JobResponse, cfg.Config.Concurrency),
	}
}

// Run starts the pool's poller and workers. It blocks until the context is
// cancelled, then waits for all in-flight jobs to complete.
func (p *Pool) Run(ctx context.Context, checkInterval time.Duration) error {
	p.logger.InfoContext(ctx, "pool starting",
		slog.Int("concurrency", p.cfg.Concurrency),
		slog.String("executor", p.cfg.Executor),
	)

	var wg sync.WaitGroup

	// Start the poller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller := NewPoller(p.client, p.runnerID, checkInterval, p.jobs, p.logger)
		poller.Run(ctx)
	}()

	// Start worker goroutines. Each worker loops, pulling jobs from the channel.
	for i := 0; i < p.cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.runWorker(ctx, workerID)
		}(i)
	}

	wg.Wait()
	p.logger.InfoContext(ctx, "pool stopped")
	return nil
}

// ActiveJobs returns the number of currently executing jobs.
func (p *Pool) ActiveJobs() int64 {
	return p.activeJobs.Load()
}

// Name returns the pool name.
func (p *Pool) Name() string {
	return p.name
}

// runWorker runs a single worker loop that pulls jobs from the channel.
func (p *Pool) runWorker(ctx context.Context, workerID int) {
	logger := p.logger.With(slog.Int("worker_id", workerID))
	logger.InfoContext(ctx, "worker started")

	for {
		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "worker stopping")
			return
		case jobResp, ok := <-p.jobs:
			if !ok {
				logger.InfoContext(ctx, "jobs channel closed, worker stopping")
				return
			}
			p.activeJobs.Add(1)

			masker := secret.NewMasker(p.output)
			exec, err := executor.New(p.cfg.Executor, p.executorConfig())
			if err != nil {
				logger.ErrorContext(ctx, "failed to create executor",
					slog.String("error", err.Error()),
				)
				p.activeJobs.Add(-1)
				continue
			}

			worker := NewWorker(WorkerConfig{
				ID:       workerID,
				RunnerID: p.runnerID,
				Executor: exec,
				Client:   p.client,
				Hooks:    p.hooks,
				Masker:   masker,
				Output:   masker,
				Logger:   logger,
			})

			if err := worker.Execute(ctx, jobResp); err != nil {
				logger.ErrorContext(ctx, "job execution failed",
					slog.Int64("job_id", jobResp.ID),
					slog.String("error", err.Error()),
				)
			}
			p.activeJobs.Add(-1)
		}
	}
}

// executorConfig returns the executor-specific configuration based on the
// runner's executor type.
func (p *Pool) executorConfig() interface{} {
	switch p.cfg.Executor {
	case "docker":
		return p.cfg.Docker
	case "kubernetes":
		return p.cfg.Kubernetes
	case "shell":
		return struct {
			WorkDir string
			Shell   string
		}{
			WorkDir: p.cfg.WorkDir,
			Shell:   p.cfg.Shell,
		}
	default:
		return fmt.Sprintf("unsupported executor: %s", p.cfg.Executor)
	}
}

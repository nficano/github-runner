package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/org/github-runner/internal/github"
)

// Poller polls the GitHub API for available jobs at a configured interval.
// When a job is found, it sends it on the jobs channel for a worker to pick up.
type Poller struct {
	client       github.GitHubClient
	runnerID     int64
	interval     time.Duration
	jobs         chan<- *github.JobResponse
	logger       *slog.Logger
}

// NewPoller creates a new job poller.
func NewPoller(
	client github.GitHubClient,
	runnerID int64,
	interval time.Duration,
	jobs chan<- *github.JobResponse,
	logger *slog.Logger,
) *Poller {
	return &Poller{
		client:   client,
		runnerID: runnerID,
		interval: interval,
		jobs:     jobs,
		logger:   logger,
	}
}

// Run starts polling for jobs. It blocks until the context is cancelled.
// Errors during polling are logged but do not stop the poller — it backs
// off and retries on the next interval.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.InfoContext(ctx, "poller started",
		slog.Duration("interval", p.interval),
		slog.Int64("runner_id", p.runnerID),
	)

	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			p.logger.InfoContext(ctx, "poller stopped")
			return
		case <-ticker.C:
			job, err := p.client.AcquireJob(ctx, p.runnerID)
			if err != nil {
				consecutiveErrors++
				p.logger.WarnContext(ctx, "poll failed",
					slog.String("error", err.Error()),
					slog.Int("consecutive_errors", consecutiveErrors),
				)
				// Back off on repeated errors: double the interval up to 5 minutes.
				if consecutiveErrors > 3 {
					backoff := p.interval * time.Duration(consecutiveErrors)
					if backoff > 5*time.Minute {
						backoff = 5 * time.Minute
					}
					ticker.Reset(backoff)
				}
				continue
			}

			// Reset error count and interval on successful poll.
			if consecutiveErrors > 0 {
				consecutiveErrors = 0
				ticker.Reset(p.interval)
			}

			if job == nil {
				// No job available — this is normal.
				continue
			}

			p.logger.InfoContext(ctx, "job acquired",
				slog.Int64("job_id", job.ID),
				slog.String("name", job.Name),
			)

			// Send job to worker pool. If the channel is full (all workers busy),
			// block until a worker is available or context is cancelled.
			select {
			case p.jobs <- job:
			case <-ctx.Done():
				return
			}
		}
	}
}

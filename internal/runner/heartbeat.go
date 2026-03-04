package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/nficano/github-runner/internal/github"
)

const defaultHeartbeatInterval = 10 * time.Second

// HeartbeatReporter sends periodic heartbeats to GitHub while a job is
// running. It runs in its own goroutine and stops when the context is cancelled.
type HeartbeatReporter struct {
	client   github.GitHubClient
	runnerID int64
	jobID    int64
	interval time.Duration
	logger   *slog.Logger
}

// NewHeartbeatReporter creates a new heartbeat reporter.
func NewHeartbeatReporter(client github.GitHubClient, runnerID, jobID int64, logger *slog.Logger) *HeartbeatReporter {
	return &HeartbeatReporter{
		client:   client,
		runnerID: runnerID,
		jobID:    jobID,
		interval: defaultHeartbeatInterval,
		logger:   logger,
	}
}

// Run sends heartbeats at regular intervals until the context is cancelled.
// This method blocks and should be called in a goroutine.
func (h *HeartbeatReporter) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.client.SendHeartbeat(ctx, h.runnerID, h.jobID); err != nil {
				h.logger.WarnContext(ctx, "heartbeat failed",
					slog.Int64("job_id", h.jobID),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

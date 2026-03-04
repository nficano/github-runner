package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/org/github-runner/pkg/api"
)

// RegistrationService provides higher-level runner registration workflows
// built on top of the GitHubClient interface.
type RegistrationService struct {
	client GitHubClient
	logger *slog.Logger
}

// NewRegistrationService creates a new registration service.
func NewRegistrationService(client GitHubClient, logger *slog.Logger) *RegistrationService {
	return &RegistrationService{
		client: client,
		logger: logger,
	}
}

// Register registers a new runner and returns its configuration.
func (s *RegistrationService) Register(ctx context.Context, opts api.RegisterOptions) (*api.RunnerConfig, error) {
	s.logger.InfoContext(ctx, "registering runner",
		slog.String("name", opts.Name),
		slog.String("url", opts.URL),
		slog.String("executor", opts.Executor),
	)

	resp, err := s.client.RegisterRunner(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("registering runner: %w", err)
	}

	labels := make([]string, len(resp.Labels))
	for i, l := range resp.Labels {
		labels[i] = l.Name
	}

	cfg := &api.RunnerConfig{
		ID:     resp.ID,
		Name:   resp.Name,
		Token:  resp.Token,
		URL:    opts.URL,
		Labels: labels,
	}

	s.logger.InfoContext(ctx, "runner registered",
		slog.Int64("id", cfg.ID),
		slog.String("name", cfg.Name),
	)

	return cfg, nil
}

// Unregister removes a runner from GitHub.
func (s *RegistrationService) Unregister(ctx context.Context, runnerID int64) error {
	s.logger.InfoContext(ctx, "unregistering runner",
		slog.Int64("id", runnerID),
	)

	if err := s.client.RemoveRunner(ctx, runnerID); err != nil {
		return fmt.Errorf("unregistering runner %d: %w", runnerID, err)
	}

	s.logger.InfoContext(ctx, "runner unregistered",
		slog.Int64("id", runnerID),
	)

	return nil
}

// Verify tests connectivity and authentication against the GitHub API.
func (s *RegistrationService) Verify(ctx context.Context) error {
	s.logger.InfoContext(ctx, "verifying GitHub API connectivity")

	runners, err := s.client.ListRunners(ctx)
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}

	s.logger.InfoContext(ctx, "GitHub API verification succeeded",
		slog.Int("runners", runners.TotalCount),
	)

	return nil
}

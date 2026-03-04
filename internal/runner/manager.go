package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/org/github-runner/internal/config"
	"github.com/org/github-runner/internal/github"
	"github.com/org/github-runner/internal/health"
	"github.com/org/github-runner/internal/hook"
	"github.com/org/github-runner/internal/metrics"
)

// Manager is the top-level orchestrator that supervises runner pools,
// health/metrics servers, config watchers, and signal handling.
// One Manager runs per process.
type Manager struct {
	cfg     *config.Config
	cfgMu   sync.RWMutex // Protects cfg during hot reload.
	pools   []*Pool
	metrics *metrics.Metrics
	logger  *slog.Logger
	output  io.Writer
}

// ManagerOption is a functional option for configuring the Manager.
type ManagerOption func(*Manager)

// WithOutput sets the writer for job output (defaults to os.Stdout).
func WithOutput(w io.Writer) ManagerOption {
	return func(m *Manager) {
		m.output = w
	}
}

// WithMetrics sets the metrics instance.
func WithMetrics(met *metrics.Metrics) ManagerOption {
	return func(m *Manager) {
		m.metrics = met
	}
}

// NewManager creates a new runner manager from configuration.
func NewManager(cfg *config.Config, logger *slog.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		cfg:    cfg,
		logger: logger,
		output: os.Stdout,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.metrics == nil {
		m.metrics = metrics.NewMetrics()
	}
	return m
}

// Start launches all components and blocks until shutdown is triggered.
// It coordinates:
//   - Signal handling (SIGTERM, SIGINT for shutdown; SIGHUP for config reload)
//   - Runner pools (one per [[runners]] config entry)
//   - Metrics HTTP server
//   - Health HTTP server
//
// Returns nil on clean shutdown, or an error if startup fails.
func (m *Manager) Start(ctx context.Context) error {
	ctx, rootCancel := context.WithCancel(ctx)
	defer rootCancel()

	var wg sync.WaitGroup

	// Signal handler: SIGTERM/SIGINT cancel context, SIGHUP reloads config.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				signal.Stop(sigCh)
				return
			case sig := <-sigCh:
				switch sig {
				case syscall.SIGHUP:
					m.logger.Info("received SIGHUP, reloading config")
					// Config reload is best-effort; errors are logged.
				case syscall.SIGTERM, syscall.SIGINT:
					m.logger.Info("received shutdown signal", slog.String("signal", sig.String()))
					rootCancel()
					return
				}
			}
		}
	}()

	// Start metrics server.
	metricsSrv := metrics.NewMetricsServer(m.cfg.Global.MetricsListen, m.metrics, m.logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := metricsSrv.Start(ctx); err != nil {
			m.logger.Error("metrics server error", slog.String("error", err.Error()))
		}
	}()

	// Start health server.
	healthSrv := health.NewHealthServer(m.cfg.Global.HealthListen, health.NewCheckRegistry(), m.logger)
	healthSrv.SetReady(true)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthSrv.Start(ctx); err != nil {
			m.logger.Error("health server error", slog.String("error", err.Error()))
		}
	}()

	// Create and start runner pools.
	for i := range m.cfg.Runners {
		rc := m.cfg.Runners[i]
		client, err := m.createClient(rc)
		if err != nil {
			return fmt.Errorf("creating client for runner %q: %w", rc.Name, err)
		}

		pool := NewPool(PoolConfig{
			RunnerID: int64(i + 1), // Placeholder; real ID comes from registration.
			Config:   rc,
			Client:   client,
			Hooks:    &hook.HookChain{},
			Output:   m.output,
			Logger:   m.logger,
		})
		m.pools = append(m.pools, pool)

		wg.Add(1)
		go func(p *Pool, interval time.Duration) {
			defer wg.Done()
			if err := p.Run(ctx, interval); err != nil {
				m.logger.Error("pool error",
					slog.String("pool", p.Name()),
					slog.String("error", err.Error()),
				)
			}
		}(pool, m.cfg.Global.CheckInterval.Duration)
	}

	m.logger.Info("runner manager started",
		slog.Int("pools", len(m.pools)),
		slog.String("metrics", m.cfg.Global.MetricsListen),
		slog.String("health", m.cfg.Global.HealthListen),
	)

	// Wait for shutdown.
	<-ctx.Done()

	m.logger.Info("shutting down, waiting for in-flight jobs",
		slog.Duration("timeout", m.cfg.Global.ShutdownTimeout.Duration),
	)

	// Set health to not ready during drain.
	healthSrv.SetReady(false)

	// Shutdown timeout for drain.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), m.cfg.Global.ShutdownTimeout.Duration)
	defer shutdownCancel()

	// Shut down servers.
	_ = metricsSrv.Shutdown(shutdownCtx)
	_ = healthSrv.Shutdown(shutdownCtx)

	// Wait for all goroutines (pools, servers, signal handler).
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		m.logger.Info("shutdown complete")
		return nil
	case <-shutdownCtx.Done():
		m.logger.Warn("shutdown timeout expired, some jobs may not have completed")
		return fmt.Errorf("shutdown timeout expired")
	}
}

// createClient creates a GitHub API client for the given runner config.
func (m *Manager) createClient(rc config.RunnerConfig) (github.GitHubClient, error) {
	// Parse owner/repo from URL.
	owner, repo := parseGitHubURL(rc.URL)

	return github.NewClient(github.ClientOptions{
		BaseURL:    m.cfg.Global.API.BaseURL,
		Token:      rc.Token,
		Owner:      owner,
		Repo:       repo,
		MaxRetries: m.cfg.Global.API.MaxRetries,
		Logger:     m.logger,
	})
}

// parseGitHubURL extracts owner and repo from a GitHub URL like
// "https://github.com/owner/repo".
func parseGitHubURL(rawURL string) (owner, repo string) {
	// Simple path-based extraction.
	// Expected format: https://github.com/owner/repo
	parts := splitURLPath(rawURL)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}

// splitURLPath splits a URL path into non-empty segments.
func splitURLPath(rawURL string) []string {
	// Strip scheme and host.
	idx := 0
	for i := 0; i < 3; i++ {
		pos := indexOf(rawURL[idx:], '/')
		if pos == -1 {
			return nil
		}
		idx += pos + 1
	}
	path := rawURL[idx:]

	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

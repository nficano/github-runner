package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nficano/github-runner/internal/config"
	"github.com/nficano/github-runner/internal/runner"
)

// newStartCmd creates the "start" subcommand, which loads the configuration,
// validates it, creates a runner Manager, and begins processing jobs.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the runner and begin processing jobs",
		Long: `Start the self-hosted GitHub Actions runner. This loads the configuration
file, validates it, and launches one worker pool per configured runner
entry. The process blocks until a shutdown signal is received.`,
		RunE: runStart,
	}

	cmd.Flags().Int("concurrency", 0, "override concurrency for all runner pools (0 = use config)")
	cmd.Flags().String("listen", "", "override health/metrics listen address")
	cmd.Flags().String("pid-file", "", "write process ID to this file")
	cmd.Flags().Bool("foreground", true, "run in the foreground (default true)")

	return cmd
}

func runStart(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	listen, _ := cmd.Flags().GetString("listen")
	pidFile, _ := cmd.Flags().GetString("pid-file")

	// Load configuration.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	// Apply overrides.
	if concurrency > 0 {
		for i := range cfg.Runners {
			cfg.Runners[i].Concurrency = concurrency
		}
	}
	if listen != "" {
		cfg.Global.HealthListen = listen
		cfg.Global.MetricsListen = listen
	}

	// Validate configuration.
	if err := config.Validate(cfg); err != nil {
		return newExitError(ExitConfigErr, "invalid configuration: %v", err)
	}

	// Write PID file if requested.
	if pidFile != "" {
		if err := writePIDFile(pidFile); err != nil {
			return newExitError(ExitError, "writing pid file: %v", err)
		}
		defer func() {
			if removeErr := os.Remove(pidFile); removeErr != nil {
				slog.Warn("failed to remove pid file",
					slog.String("path", pidFile),
					slog.String("error", removeErr.Error()),
				)
			}
		}()
	}

	logger := slog.Default()
	mgr := runner.NewManager(cfg, logger)

	slog.Info("starting runner",
		slog.String("config", cfgPath),
		slog.Int("pools", len(cfg.Runners)),
	)

	if err := mgr.Start(context.Background()); err != nil {
		return newExitError(ExitError, "runner manager error: %v", err)
	}

	return nil
}

// writePIDFile writes the current process ID to the specified path.
func writePIDFile(path string) error {
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing pid %d to %s: %w", pid, path, err)
	}
	slog.Debug("pid file written", slog.String("path", path), slog.Int("pid", pid))
	return nil
}

package cli

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/org/github-runner/internal/config"
	"github.com/org/github-runner/internal/runner"
)

// newRunCmd creates the "run" subcommand, which executes a single job
// and then exits. This is useful for ephemeral or one-shot runner modes.
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single job and exit",
		Long: `Execute a single job from the queue and exit. The --runner flag selects
which configured runner pool to use. When --once is set (the default),
the process exits after completing one job.`,
		RunE: runRun,
	}

	cmd.Flags().String("runner", "", "name of the runner pool to use (defaults to first configured)")
	cmd.Flags().Bool("once", true, "exit after completing one job")

	return cmd
}

func runRun(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	runnerName, _ := cmd.Flags().GetString("runner")
	once, _ := cmd.Flags().GetBool("once")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	if err := config.Validate(cfg); err != nil {
		return newExitError(ExitConfigErr, "invalid configuration: %v", err)
	}

	// Find the target runner configuration.
	rc, err := findRunner(cfg, runnerName)
	if err != nil {
		return newExitError(ExitConfigErr, "%v", err)
	}

	// Force ephemeral/once mode.
	if once {
		rc.Ephemeral = true
		rc.Concurrency = 1
	}

	logger := slog.Default()

	// Build a single-runner config for the manager.
	singleCfg := *cfg
	singleCfg.Runners = []config.RunnerConfig{*rc}

	mgr := runner.NewManager(&singleCfg, logger)

	slog.Info("starting single-job run",
		slog.String("runner", rc.Name),
		slog.Bool("once", once),
	)

	if err := mgr.Start(context.Background()); err != nil {
		return newExitError(ExitError, "run failed: %v", err)
	}

	return nil
}

// findRunner locates a runner configuration by name. If name is empty,
// the first configured runner is returned.
func findRunner(cfg *config.Config, name string) (*config.RunnerConfig, error) {
	if len(cfg.Runners) == 0 {
		return nil, newExitError(ExitConfigErr, "no runners configured")
	}

	if name == "" {
		return &cfg.Runners[0], nil
	}

	for i := range cfg.Runners {
		if cfg.Runners[i].Name == name {
			return &cfg.Runners[i], nil
		}
	}

	return nil, newExitError(ExitConfigErr, "runner %q not found in configuration", name)
}

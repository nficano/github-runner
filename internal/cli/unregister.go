package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/nficano/github-runner/internal/config"
	"github.com/nficano/github-runner/internal/github"
)

// newUnregisterCmd creates the "unregister" subcommand, which removes a
// runner from GitHub and deletes it from the local configuration file.
func newUnregisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unregister",
		Short: "Remove a runner from GitHub",
		Long: `Remove a registered runner from GitHub and delete the corresponding
entry from the local configuration file.`,
		RunE: runUnregister,
	}

	cmd.Flags().String("name", "", "name of the runner to unregister")
	cmd.Flags().String("token", "", "authentication token (overrides config)")
	cmd.Flags().Bool("all-runners", false, "unregister all configured runners")

	return cmd
}

func runUnregister(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	tokenOverride, _ := cmd.Flags().GetString("token")
	allRunners, _ := cmd.Flags().GetBool("all-runners")

	if name == "" && !allRunners {
		return newExitError(ExitError, "either --name or --all-runners must be specified")
	}

	cfgPath := configPath(cmd)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	if len(cfg.Runners) == 0 {
		return newExitError(ExitConfigErr, "no runners configured in %s", cfgPath)
	}

	var remaining []config.RunnerConfig

	for i := range cfg.Runners {
		rc := cfg.Runners[i]

		if !allRunners && rc.Name != name {
			remaining = append(remaining, rc)
			continue
		}

		token := rc.Token
		if tokenOverride != "" {
			token = tokenOverride
		}

		if err := removeRunner(cfg, rc, token); err != nil {
			if isAuthError(err) {
				return newExitError(ExitAuthErr, "authentication failed for runner %q: %v", rc.Name, err)
			}
			return newExitError(ExitError, "removing runner %q: %v", rc.Name, err)
		}

		slog.Info("runner unregistered", slog.String("name", rc.Name))
		fmt.Fprintf(os.Stdout, "Runner %q unregistered\n", rc.Name)
	}

	// Update config file with remaining runners.
	cfg.Runners = remaining
	if err := saveConfig(cfgPath, cfg); err != nil {
		return newExitError(ExitConfigErr, "saving config: %v", err)
	}

	return nil
}

// removeRunner creates a client and calls RemoveRunner for the given runner
// configuration. The token parameter may override the config token.
func removeRunner(cfg *config.Config, rc config.RunnerConfig, token string) error {
	owner, repo := parseRegistrationURL(rc.URL)

	client, err := github.NewClient(github.ClientOptions{
		BaseURL: cfg.Global.API.BaseURL,
		Token:   token,
		Owner:   owner,
		Repo:    repo,
		Logger:  slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("creating github client: %w", err)
	}

	// We need the runner ID. List runners and match by name.
	runners, err := client.ListRunners(context.Background())
	if err != nil {
		return fmt.Errorf("listing runners: %w", err)
	}

	var runnerID int64
	for _, r := range runners.Runners {
		if r.Name == rc.Name {
			runnerID = r.ID
			break
		}
	}
	if runnerID == 0 {
		slog.Warn("runner not found on GitHub, removing from local config only",
			slog.String("name", rc.Name),
		)
		return nil
	}

	if err := client.RemoveRunner(context.Background(), runnerID); err != nil {
		return fmt.Errorf("removing runner %d: %w", runnerID, err)
	}
	return nil
}

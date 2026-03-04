package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/org/github-runner/internal/config"
	"github.com/org/github-runner/internal/github"
)

// newVerifyCmd creates the "verify" subcommand, which tests connectivity
// and authentication against the GitHub API for configured runners.
func newVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify connectivity and authentication with GitHub",
		Long: `Test that each configured runner can successfully connect to the
GitHub API and authenticate. This is useful for validating tokens and
network connectivity after registration.`,
		RunE: runVerify,
	}

	cmd.Flags().String("runner", "", "verify a specific runner by name (default: all)")

	return cmd
}

func runVerify(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	runnerName, _ := cmd.Flags().GetString("runner")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	if len(cfg.Runners) == 0 {
		return newExitError(ExitConfigErr, "no runners configured in %s", cfgPath)
	}

	var targets []config.RunnerConfig
	if runnerName != "" {
		found := false
		for _, rc := range cfg.Runners {
			if rc.Name == runnerName {
				targets = append(targets, rc)
				found = true
				break
			}
		}
		if !found {
			return newExitError(ExitConfigErr, "runner %q not found in configuration", runnerName)
		}
	} else {
		targets = cfg.Runners
	}

	hasErrors := false
	for _, rc := range targets {
		if err := verifyRunner(cfg, rc); err != nil {
			hasErrors = true
			slog.Error("verification failed",
				slog.String("runner", rc.Name),
				slog.String("error", err.Error()),
			)
			fmt.Fprintf(os.Stdout, "FAIL  %s: %v\n", rc.Name, err)
		} else {
			slog.Info("verification succeeded", slog.String("runner", rc.Name))
			fmt.Fprintf(os.Stdout, "OK    %s\n", rc.Name)
		}
	}

	if hasErrors {
		return newExitError(ExitAuthErr, "one or more runners failed verification")
	}

	return nil
}

// verifyRunner tests connectivity for a single runner configuration by
// listing runners through the GitHub API.
func verifyRunner(cfg *config.Config, rc config.RunnerConfig) error {
	owner, repo := parseRegistrationURL(rc.URL)

	client, err := github.NewClient(github.ClientOptions{
		BaseURL: cfg.Global.API.BaseURL,
		Token:   rc.Token,
		Owner:   owner,
		Repo:    repo,
		Logger:  slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	_, err = client.ListRunners(context.Background())
	if err != nil {
		return fmt.Errorf("listing runners: %w", err)
	}

	return nil
}

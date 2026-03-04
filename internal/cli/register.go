package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/nficano/github-runner/internal/config"
	"github.com/nficano/github-runner/internal/github"
	"github.com/nficano/github-runner/pkg/api"
)

// newRegisterCmd creates the "register" subcommand, which registers the
// runner with a GitHub repository or organisation and persists the result
// to the configuration file.
func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new runner with GitHub",
		Long: `Register a new self-hosted runner with a GitHub repository or organisation.
On success the runner configuration is appended to the config file.`,
		RunE: runRegister,
	}

	cmd.Flags().String("url", "", "GitHub repository or organisation URL (required)")
	cmd.Flags().String("token", "", "one-time registration token (required)")
	cmd.Flags().String("name", "", "runner name (defaults to hostname)")
	cmd.Flags().String("executor", "", "executor type: shell, docker, kubernetes, firecracker (required)")
	cmd.Flags().StringSlice("labels", nil, "comma-separated list of custom labels")
	cmd.Flags().String("work-dir", "", "root directory for job workspaces")
	cmd.Flags().Bool("ephemeral", false, "de-register after a single job")

	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("token")
	_ = cmd.MarkFlagRequired("executor")

	return cmd
}

func runRegister(cmd *cobra.Command, _ []string) error {
	url, _ := cmd.Flags().GetString("url")
	token, _ := cmd.Flags().GetString("token")
	name, _ := cmd.Flags().GetString("name")
	executor, _ := cmd.Flags().GetString("executor")
	labels, _ := cmd.Flags().GetStringSlice("labels")
	workDir, _ := cmd.Flags().GetString("work-dir")
	ephemeral, _ := cmd.Flags().GetBool("ephemeral")

	// Default name to hostname.
	if name == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return newExitError(ExitError, "determining hostname: %v", err)
		}
		name = hostname
	}

	cfgPath := configPath(cmd)

	// Load existing config (or start from defaults if file does not exist).
	cfg, err := loadOrDefaultConfig(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config: %v", err)
	}

	// Build a GitHub client for registration.
	owner, repo := parseRegistrationURL(url)
	client, err := github.NewClient(github.ClientOptions{
		BaseURL: cfg.Global.API.BaseURL,
		Token:   token,
		Owner:   owner,
		Repo:    repo,
		Logger:  slog.Default(),
	})
	if err != nil {
		return newExitError(ExitError, "creating github client: %v", err)
	}

	slog.Info("registering runner",
		slog.String("name", name),
		slog.String("url", url),
		slog.String("executor", executor),
	)

	resp, err := client.RegisterRunner(context.Background(), api.RegisterOptions{
		URL:       url,
		Token:     token,
		Name:      name,
		Labels:    labels,
		Executor:  executor,
		WorkDir:   workDir,
		Ephemeral: ephemeral,
	})
	if err != nil {
		if isAuthError(err) {
			return newExitError(ExitAuthErr, "authentication failed: %v", err)
		}
		return newExitError(ExitError, "registering runner: %v", err)
	}

	// Append the new runner to config and persist.
	rc := config.RunnerConfig{
		Name:      resp.Name,
		URL:       url,
		Token:     resp.Token,
		Executor:  executor,
		Labels:    labels,
		WorkDir:   workDir,
		Ephemeral: ephemeral,
	}
	cfg.Runners = append(cfg.Runners, rc)

	if err := saveConfig(cfgPath, cfg); err != nil {
		return newExitError(ExitConfigErr, "saving config: %v", err)
	}

	slog.Info("runner registered successfully",
		slog.Int64("id", resp.ID),
		slog.String("name", resp.Name),
	)
	fmt.Fprintf(os.Stdout, "Runner %q registered (id=%d)\n", resp.Name, resp.ID)
	return nil
}

// loadOrDefaultConfig loads the config file at path, or returns a default
// config if the file does not exist.
func loadOrDefaultConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config.DefaultConfig(), nil
		}
		return nil, fmt.Errorf("loading config from %s: %w", path, err)
	}
	return cfg, nil
}

// saveConfig writes the config to the given path in TOML format.
func saveConfig(path string, cfg *config.Config) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file %s: %w", path, err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config to %s: %w", path, err)
	}
	return nil
}

// parseRegistrationURL extracts the owner and optional repo from a GitHub URL.
func parseRegistrationURL(rawURL string) (owner, repo string) {
	// Reuse the same logic as runner/manager.go but inline a simple version.
	// Expected format: https://github.com/owner[/repo]
	parts := splitPath(rawURL)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}

// splitPath strips the scheme+host from a URL and returns the non-empty path
// segments.
func splitPath(rawURL string) []string {
	// Find the third slash (after scheme://host).
	slashes := 0
	idx := 0
	for i, c := range rawURL {
		if c == '/' {
			slashes++
			if slashes == 3 {
				idx = i + 1
				break
			}
		}
	}
	if slashes < 3 {
		return nil
	}

	var parts []string
	current := ""
	for _, c := range rawURL[idx:] {
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

// isAuthError returns true if the error represents a 401 or 403 response
// from the GitHub API.
func isAuthError(err error) bool {
	// Walk the error chain looking for an APIError with an auth status code.
	current := err
	for current != nil {
		if ae, ok := current.(*github.APIError); ok {
			return ae.StatusCode == http.StatusUnauthorized || ae.StatusCode == http.StatusForbidden
		}
		if unwrapper, ok := current.(interface{ Unwrap() error }); ok {
			current = unwrapper.Unwrap()
		} else {
			break
		}
	}
	return false
}

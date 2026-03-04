package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/nficano/github-runner/internal/config"
	"github.com/nficano/github-runner/internal/github"
)

// newStatusCmd creates the "status" subcommand, which displays the live
// status of configured runners including active jobs.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show runner status and active jobs",
		Long: `Display the current status of all configured runners, including whether
they are online, idle, or actively processing jobs. Use --watch to
continuously refresh.`,
		RunE: runStatus,
	}

	cmd.Flags().String("format", "table", "output format: table, json")
	cmd.Flags().Bool("watch", false, "continuously refresh status")

	return cmd
}

// statusRow holds the status information for a single runner.
type statusRow struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Busy   bool   `json:"busy"`
	OS     string `json:"os"`
	Labels string `json:"labels"`
}

func runStatus(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	format, _ := cmd.Flags().GetString("format")
	watch, _ := cmd.Flags().GetBool("watch")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	if len(cfg.Runners) == 0 {
		return newExitError(ExitConfigErr, "no runners configured in %s", cfgPath)
	}

	// If --watch is set, loop until interrupted.
	if watch {
		return watchStatus(cmd.Context(), cfg, format)
	}

	rows, err := fetchStatus(cfg)
	if err != nil {
		return err
	}
	return formatStatus(rows, format)
}

// watchStatus continuously polls and displays runner status until the
// context is cancelled.
func watchStatus(ctx context.Context, cfg *config.Config, format string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		// Clear screen for table format.
		if format == "table" {
			fmt.Fprint(os.Stdout, "\033[2J\033[H")
		}

		rows, err := fetchStatus(cfg)
		if err != nil {
			slog.Warn("failed to fetch status", slog.String("error", err.Error()))
		} else {
			if err := formatStatus(rows, format); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// fetchStatus queries the GitHub API for runner status across all
// configured runner entries.
func fetchStatus(cfg *config.Config) ([]statusRow, error) {
	seen := make(map[int64]bool)
	var rows []statusRow

	for _, rc := range cfg.Runners {
		owner, repo := parseRegistrationURL(rc.URL)
		client, err := github.NewClient(github.ClientOptions{
			BaseURL: cfg.Global.API.BaseURL,
			Token:   rc.Token,
			Owner:   owner,
			Repo:    repo,
			Logger:  slog.Default(),
		})
		if err != nil {
			continue
		}

		list, err := client.ListRunners(context.Background())
		if err != nil {
			slog.Warn("failed to query runner status",
				slog.String("runner", rc.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		for _, r := range list.Runners {
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true

			var labels []string
			for _, l := range r.Labels {
				labels = append(labels, l.Name)
			}
			rows = append(rows, statusRow{
				Name:   r.Name,
				Status: r.Status,
				Busy:   r.Busy,
				OS:     r.OS,
				Labels: strings.Join(labels, ","),
			})
		}
	}

	return rows, nil
}

// formatStatus writes status rows to stdout in the requested format.
func formatStatus(rows []statusRow, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return newExitError(ExitError, "encoding json: %v", err)
		}
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tBUSY\tOS\tLABELS")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n",
				r.Name, r.Status, r.Busy, r.OS, r.Labels)
		}
		if err := w.Flush(); err != nil {
			return newExitError(ExitError, "flushing table output: %v", err)
		}
	default:
		return newExitError(ExitConfigErr, "unsupported format %q: must be table or json", format)
	}
	return nil
}

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/org/github-runner/internal/config"
	"github.com/org/github-runner/internal/github"
)

// newListCmd creates the "list" subcommand, which lists all registered
// runners and their current status.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered runners and their status",
		Long: `Query GitHub for all runners visible to each configured runner entry
and display their name, status, labels, and activity.`,
		RunE: runList,
	}

	cmd.Flags().String("format", "table", "output format: table, json, yaml")

	return cmd
}

// runnerRow is a flat representation of runner data for output formatting.
type runnerRow struct {
	ID     int64    `json:"id" yaml:"id"`
	Name   string   `json:"name" yaml:"name"`
	OS     string   `json:"os" yaml:"os"`
	Status string   `json:"status" yaml:"status"`
	Busy   bool     `json:"busy" yaml:"busy"`
	Labels []string `json:"labels" yaml:"labels"`
}

func runList(cmd *cobra.Command, _ []string) error {
	cfgPath := configPath(cmd)
	format, _ := cmd.Flags().GetString("format")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return newExitError(ExitConfigErr, "loading config from %s: %v", cfgPath, err)
	}

	if len(cfg.Runners) == 0 {
		return newExitError(ExitConfigErr, "no runners configured in %s", cfgPath)
	}

	// Collect runners from all configured entries, deduplicating by ID.
	seen := make(map[int64]bool)
	var rows []runnerRow

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
			slog.Warn("skipping runner, failed to create client",
				slog.String("name", rc.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		list, err := client.ListRunners(context.Background())
		if err != nil {
			slog.Warn("failed to list runners",
				slog.String("name", rc.Name),
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
			rows = append(rows, runnerRow{
				ID:     r.ID,
				Name:   r.Name,
				OS:     r.OS,
				Status: r.Status,
				Busy:   r.Busy,
				Labels: labels,
			})
		}
	}

	return formatRunnerList(rows, format)
}

// formatRunnerList writes the runner list in the requested format to stdout.
func formatRunnerList(rows []runnerRow, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return newExitError(ExitError, "encoding json: %v", err)
		}
	case "yaml":
		// Simple YAML output without pulling in a yaml dependency.
		for _, r := range rows {
			fmt.Fprintf(os.Stdout, "- id: %d\n  name: %s\n  os: %s\n  status: %s\n  busy: %v\n  labels: [%s]\n",
				r.ID, r.Name, r.OS, r.Status, r.Busy, strings.Join(r.Labels, ", "))
		}
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tOS\tSTATUS\tBUSY\tLABELS")
		for _, r := range rows {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%v\t%s\n",
				r.ID, r.Name, r.OS, r.Status, r.Busy, strings.Join(r.Labels, ","))
		}
		if err := w.Flush(); err != nil {
			return newExitError(ExitError, "flushing table output: %v", err)
		}
	default:
		return newExitError(ExitConfigErr, "unsupported format %q: must be table, json, or yaml", format)
	}
	return nil
}

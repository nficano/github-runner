package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nficano/github-runner/internal/version"
)

// newVersionCmd creates the "version" subcommand, which prints build
// version information. Use --format json for machine-readable output.
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Display the build version, commit SHA, build date, Go version, and platform.`,
		RunE:  runVersion,
	}

	cmd.Flags().String("format", "text", "output format: text, json")

	return cmd
}

func runVersion(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString("format")
	info := version.Get()

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			return newExitError(ExitError, "encoding version as json: %v", err)
		}
	case "text":
		fmt.Fprintln(os.Stdout, info.String())
	default:
		return newExitError(ExitConfigErr, "unsupported format %q: must be text or json", format)
	}

	return nil
}

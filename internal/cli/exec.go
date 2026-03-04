package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newExecCmd creates the "exec" subcommand, which provides local workflow
// execution for development and testing. This is a scaffold that will be
// implemented in a future release.
func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a workflow locally (experimental)",
		Long: `Run a GitHub Actions workflow file locally for development and testing
purposes. This command parses the workflow YAML and executes jobs using
the configured executor.

NOTE: This feature is not yet implemented.`,
		RunE: runExec,
	}

	cmd.Flags().String("workflow", "", "path to the workflow YAML file")
	cmd.Flags().String("job", "", "name of the job to execute (default: all)")
	cmd.Flags().String("event", "push", "simulated event type")
	cmd.Flags().StringSlice("secret", nil, "secret in KEY=VALUE format (can be repeated)")
	cmd.Flags().StringSlice("env", nil, "environment variable in KEY=VALUE format (can be repeated)")

	return cmd
}

func runExec(_ *cobra.Command, _ []string) error {
	fmt.Fprintln(os.Stderr, "local exec not yet implemented")
	return newExitError(ExitError, "local exec not yet implemented")
}

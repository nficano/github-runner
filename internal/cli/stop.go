package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// newStopCmd creates the "stop" subcommand, which reads a PID file and
// sends a signal to gracefully (or forcefully) shut down the runner process.
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running runner process",
		Long: `Stop a running github-runner process by reading its PID file and
sending SIGTERM. Use --force to send SIGKILL instead.`,
		RunE: runStop,
	}

	cmd.Flags().String("pid-file", "/var/run/github-runner.pid", "path to the PID file")
	cmd.Flags().Bool("force", false, "send SIGKILL instead of SIGTERM")

	return cmd
}

func runStop(cmd *cobra.Command, _ []string) error {
	pidFile, _ := cmd.Flags().GetString("pid-file")
	force, _ := cmd.Flags().GetBool("force")

	pid, err := readPIDFile(pidFile)
	if err != nil {
		return newExitError(ExitError, "reading pid file: %v", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return newExitError(ExitError, "finding process %d: %v", pid, err)
	}

	sig := syscall.SIGTERM
	sigName := "SIGTERM"
	if force {
		sig = syscall.SIGKILL
		sigName = "SIGKILL"
	}

	slog.Info("sending signal to runner process",
		slog.Int("pid", pid),
		slog.String("signal", sigName),
	)

	if err := proc.Signal(sig); err != nil {
		return newExitError(ExitError, "sending %s to pid %d: %v", sigName, pid, err)
	}

	fmt.Fprintf(os.Stdout, "Sent %s to process %d\n", sigName, pid)
	return nil
}

// readPIDFile reads and parses the PID from the given file path.
func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading pid file %s: %w", path, err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("parsing pid from %s: invalid content %q: %w", path, pidStr, err)
	}

	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d in %s", pid, path)
	}

	return pid, nil
}

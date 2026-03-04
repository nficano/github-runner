// Package cli implements the cobra-based command-line interface for the
// self-hosted GitHub Actions runner. It wires together configuration loading,
// structured logging, and all subcommands.
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Exit codes used by all commands.
const (
	ExitSuccess    = 0
	ExitError      = 1
	ExitConfigErr  = 2
	ExitAuthErr    = 3
)

// rootCmd is the top-level command for the github-runner binary.
var rootCmd = &cobra.Command{
	Use:   "github-runner",
	Short: "Self-hosted GitHub Actions runner",
	Long: `A self-hosted GitHub Actions runner that supports shell, Docker,
Kubernetes, and Firecracker executors with hot-reload, caching, and
structured logging.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		return setupLogging(cmd)
	},
}

// Execute runs the root command and returns the exit code. The caller
// (main.go) should pass this value to os.Exit.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		// If the error carries a specific exit code, use it.
		if coded, ok := err.(*exitError); ok {
			slog.Error(coded.Error())
			return coded.code
		}
		slog.Error(err.Error())
		return ExitError
	}
	return ExitSuccess
}

// exitError wraps an error with a specific process exit code.
type exitError struct {
	err  error
	code int
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

// newExitError creates an exitError with the given code and message.
func newExitError(code int, format string, args ...any) *exitError {
	return &exitError{
		err:  fmt.Errorf(format, args...),
		code: code,
	}
}

// setupLogging configures the global slog logger based on the --log-level
// and --log-format persistent flags.
func setupLogging(cmd *cobra.Command) error {
	levelStr, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return fmt.Errorf("reading log-level flag: %w", err)
	}
	formatStr, err := cmd.Flags().GetString("log-format")
	if err != nil {
		return fmt.Errorf("reading log-format flag: %w", err)
	}

	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return newExitError(ExitConfigErr, "invalid log level %q: must be debug, info, warn, or error", levelStr)
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch strings.ToLower(formatStr) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		return newExitError(ExitConfigErr, "invalid log format %q: must be json or text", formatStr)
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

// configPath returns the --config flag value from the command.
func configPath(cmd *cobra.Command) string {
	path, _ := cmd.Flags().GetString("config")
	return path
}

func init() {
	rootCmd.PersistentFlags().String("config", "/etc/github-runner/config.toml", "path to configuration file")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "json", "log format (json, text)")

	rootCmd.AddCommand(
		newRegisterCmd(),
		newUnregisterCmd(),
		newStartCmd(),
		newStopCmd(),
		newRunCmd(),
		newListCmd(),
		newVerifyCmd(),
		newStatusCmd(),
		newCacheCmd(),
		newExecCmd(),
		newVersionCmd(),
	)
}

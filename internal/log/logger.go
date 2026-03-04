package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Setup creates a new [slog.Logger] that writes to os.Stderr.
//
// level must be one of "debug", "info", "warn", or "error" (case-insensitive).
// format must be "json" or "text" (case-insensitive).
//
// The returned logger can be further customised with [WithJobContext] or by
// calling slog.Logger.With to add more structured fields.
func Setup(level, format string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("setting up logger: %w", err)
	}

	handler, err := newHandler(lvl, format)
	if err != nil {
		return nil, fmt.Errorf("setting up logger: %w", err)
	}

	return slog.New(handler), nil
}

// SetupWithMask is like [Setup] but wraps the handler in a [MaskingHandler] so
// that every emitted log record has its string content filtered through mask
// before it reaches the output.
func SetupWithMask(level, format string, mask MaskFunc) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("setting up logger with mask: %w", err)
	}

	inner, err := newHandler(lvl, format)
	if err != nil {
		return nil, fmt.Errorf("setting up logger with mask: %w", err)
	}

	handler := NewMaskingHandler(inner, mask)
	return slog.New(handler), nil
}

// WithComponent returns a child logger tagged with the given component name.
// This is the idiomatic way to create per-subsystem loggers.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With(slog.String(FieldComponent, component))
}

// WithJobContext returns a child logger enriched with fields that identify a
// specific job execution. The returned logger should be used for all log
// output that relates to the given job.
func WithJobContext(logger *slog.Logger, jobID int64, repo, workflow string) *slog.Logger {
	return logger.With(
		slog.Int64(FieldJobID, jobID),
		slog.String(FieldRepo, repo),
		slog.String(FieldWorkflow, workflow),
	)
}

// parseLevel converts a human-readable level string to a [slog.Level].
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q: expected debug, info, warn, or error", s)
	}
}

// newHandler builds a concrete slog handler (text or JSON) at the given level.
func newHandler(level slog.Level, format string) (slog.Handler, error) {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.NewJSONHandler(os.Stderr, opts), nil
	case "text":
		return slog.NewTextHandler(os.Stderr, opts), nil
	default:
		return nil, fmt.Errorf("unknown log format %q: expected json or text", format)
	}
}

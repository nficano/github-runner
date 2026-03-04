package log

import (
	"context"
	"log/slog"
)

// MaskFunc is a function that replaces secret values in a string with a
// masked placeholder (e.g. "***"). It is provided by the caller so that the
// handler itself does not need to know which secrets exist.
type MaskFunc func(string) string

// MaskingHandler wraps another [slog.Handler] and applies a [MaskFunc] to the
// log message and all string-typed attributes before forwarding the record to
// the underlying handler. This prevents accidental secret leakage in logs.
type MaskingHandler struct {
	inner slog.Handler
	mask  MaskFunc
}

// NewMaskingHandler returns a handler that masks secrets in every record before
// delegating to inner. If mask is nil the handler passes records through
// unmodified.
func NewMaskingHandler(inner slog.Handler, mask MaskFunc) *MaskingHandler {
	return &MaskingHandler{
		inner: inner,
		mask:  mask,
	}
}

// Enabled reports whether the underlying handler is enabled for the given level.
func (h *MaskingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle masks the message and string attributes of r, then forwards the
// masked record to the underlying handler.
func (h *MaskingHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.mask != nil {
		r.Message = h.mask(r.Message)

		// Build a new set of attributes with masked string values.
		masked := make([]slog.Attr, 0, r.NumAttrs())
		r.Attrs(func(a slog.Attr) bool {
			masked = append(masked, h.maskAttr(a))
			return true
		})

		// Create a copy of the record without the original attrs so we can
		// replace them with the masked versions.
		nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		nr.AddAttrs(masked...)
		r = nr
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new MaskingHandler whose underlying handler carries the
// given pre-masked attributes.
func (h *MaskingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	masked := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		masked[i] = h.maskAttr(a)
	}
	return &MaskingHandler{
		inner: h.inner.WithAttrs(masked),
		mask:  h.mask,
	}
}

// WithGroup returns a new MaskingHandler whose underlying handler is scoped to
// the named group.
func (h *MaskingHandler) WithGroup(name string) slog.Handler {
	return &MaskingHandler{
		inner: h.inner.WithGroup(name),
		mask:  h.mask,
	}
}

// maskAttr applies the mask function to a single attribute. Group attributes
// are recursively masked; string values are masked directly; all other types
// pass through unchanged.
func (h *MaskingHandler) maskAttr(a slog.Attr) slog.Attr {
	if h.mask == nil {
		return a
	}

	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.mask(a.Value.String()))
	case slog.KindGroup:
		attrs := a.Value.Group()
		masked := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			masked[i] = h.maskAttr(ga)
		}
		return slog.Group(a.Key, attrsToAny(masked)...)
	default:
		return a
	}
}

// attrsToAny converts a slice of [slog.Attr] to []any so it can be passed to
// [slog.Group].
func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, len(attrs))
	for i, a := range attrs {
		out[i] = a
	}
	return out
}

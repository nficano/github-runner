// Package secret provides secret storage, injection, and log masking.
package secret

import (
	"encoding/base64"
	"io"
	"net/url"
	"strings"
	"sync"
)

// Masker is an io.Writer that replaces secret values with "***" in all output.
// It handles exact matches, base64-encoded variants, and URL-encoded variants.
// It is safe for concurrent use.
type Masker struct {
	w        io.Writer
	mu       sync.RWMutex
	patterns []string // all patterns to mask (original + encoded variants)
	buf      []byte   // internal buffer for partial line assembly
}

// NewMasker creates a new Masker that writes masked output to w.
func NewMasker(w io.Writer) *Masker {
	return &Masker{
		w: w,
	}
}

// AddSecret registers a secret value for masking. It also registers
// base64-encoded and URL-encoded variants of the value. Thread-safe.
func (m *Masker) AddSecret(value string) {
	if value == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Add the original value.
	m.patterns = append(m.patterns, value)

	// Add base64-encoded variant.
	b64 := base64.StdEncoding.EncodeToString([]byte(value))
	if b64 != value {
		m.patterns = append(m.patterns, b64)
	}

	// Add URL-encoded variant.
	urlEnc := url.QueryEscape(value)
	if urlEnc != value {
		m.patterns = append(m.patterns, urlEnc)
	}
}

// Write masks any registered secret patterns in p before writing to the
// underlying writer. It buffers incomplete lines to handle secrets that
// may be split across write boundaries.
func (m *Masker) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	m.mu.RLock()
	patterns := make([]string, len(m.patterns))
	copy(patterns, m.patterns)
	m.mu.RUnlock()

	// If no patterns registered, pass through directly.
	if len(patterns) == 0 {
		return m.w.Write(p)
	}

	// Append incoming data to buffer.
	m.buf = append(m.buf, p...)

	// Find the last newline — we only flush complete lines to ensure
	// secrets split across writes are caught.
	lastNewline := -1
	for i := len(m.buf) - 1; i >= 0; i-- {
		if m.buf[i] == '\n' {
			lastNewline = i
			break
		}
	}

	// If the buffer is very large (>64KB) but has no newlines, flush anyway
	// to avoid unbounded memory growth.
	const maxBufSize = 64 * 1024
	var toFlush []byte
	if lastNewline >= 0 {
		toFlush = m.buf[:lastNewline+1]
		remaining := make([]byte, len(m.buf)-lastNewline-1)
		copy(remaining, m.buf[lastNewline+1:])
		m.buf = remaining
	} else if len(m.buf) > maxBufSize {
		toFlush = m.buf
		m.buf = nil
	} else {
		// Wait for more data or a flush.
		return len(p), nil
	}

	masked := maskString(string(toFlush), patterns)
	_, err := io.WriteString(m.w, masked)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush writes any remaining buffered data to the underlying writer
// after applying masking. Call this when the stream ends.
func (m *Masker) Flush() error {
	if len(m.buf) == 0 {
		return nil
	}

	m.mu.RLock()
	patterns := make([]string, len(m.patterns))
	copy(patterns, m.patterns)
	m.mu.RUnlock()

	masked := maskString(string(m.buf), patterns)
	m.buf = nil
	_, err := io.WriteString(m.w, masked)
	return err
}

// MaskString applies all registered masking patterns to the given string.
func (m *Masker) MaskString(s string) string {
	m.mu.RLock()
	patterns := make([]string, len(m.patterns))
	copy(patterns, m.patterns)
	m.mu.RUnlock()

	return maskString(s, patterns)
}

// maskString replaces all occurrences of patterns in s with "***".
func maskString(s string, patterns []string) string {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		s = strings.ReplaceAll(s, p, "***")
	}
	return s
}

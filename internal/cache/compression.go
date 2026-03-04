package cache

import (
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// NewCompressWriter returns an io.WriteCloser that compresses data written to
// it using Zstandard (zstd) compression and writes the compressed output to w.
//
// The caller must call Close on the returned writer to flush any buffered
// compressed data and release resources. The underlying writer w is NOT closed
// by Close.
//
// Example usage:
//
//	var buf bytes.Buffer
//	cw := NewCompressWriter(&buf)
//	io.Copy(cw, src)
//	cw.Close()
//	// buf now contains zstd-compressed data.
func NewCompressWriter(w io.Writer) io.WriteCloser {
	// zstd.NewWriter only returns an error if invalid options are passed.
	// With default options it is safe to ignore the error.
	enc, err := zstd.NewWriter(w)
	if err != nil {
		// This should never happen with default options, but if it does we
		// return a writer that reports the error on first write.
		return &errWriter{err: fmt.Errorf("creating zstd encoder: %w", err)}
	}
	return enc
}

// NewDecompressReader returns an io.ReadCloser that decompresses zstd-
// compressed data from r.
//
// The caller must call Close on the returned reader when done to release
// the decoder resources. The underlying reader r is NOT closed by Close.
//
// Returns an error if the reader cannot be initialised (e.g., if r
// contains invalid zstd data that prevents decoder setup).
func NewDecompressReader(r io.Reader) (io.ReadCloser, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("creating zstd decoder: %w", err)
	}
	return &zstdReadCloser{dec: dec}, nil
}

// zstdReadCloser wraps a *zstd.Decoder to implement io.ReadCloser.
type zstdReadCloser struct {
	dec *zstd.Decoder
}

// Read implements io.Reader.
func (z *zstdReadCloser) Read(p []byte) (int, error) {
	return z.dec.Read(p)
}

// Close releases the decoder resources.
func (z *zstdReadCloser) Close() error {
	z.dec.Close()
	return nil
}

// errWriter is an io.WriteCloser that returns an error on every Write call.
// It is used as a fallback when the zstd encoder cannot be created.
type errWriter struct {
	err error
}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, e.err
}

func (e *errWriter) Close() error {
	return nil
}

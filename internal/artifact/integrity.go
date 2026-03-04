package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ComputeSHA256 reads all data from r and returns the hex-encoded SHA-256
// digest. The caller is responsible for closing r if applicable.
func ComputeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("computing SHA-256: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 reads all data from r and verifies that its SHA-256 digest
// matches the expected hex-encoded value. Returns nil if the digests match,
// or an error describing the mismatch.
func VerifySHA256(r io.Reader, expected string) error {
	actual, err := ComputeSHA256(r)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// HashingReader wraps an io.Reader to compute a SHA-256 hash of all data
// read through it. After all data has been read, call Sum to retrieve the
// hex-encoded digest.
//
// Example:
//
//	hr := NewHashingReader(file)
//	io.Copy(dst, hr)
//	digest := hr.Sum()
type HashingReader struct {
	r io.Reader
	h interface {
		io.Writer
		Sum(b []byte) []byte
	}
}

// NewHashingReader creates a new HashingReader that computes SHA-256 while
// reading from r. The underlying reader r is not closed by the HashingReader.
func NewHashingReader(r io.Reader) *HashingReader {
	return &HashingReader{
		r: r,
		h: sha256.New(),
	}
}

// Read implements io.Reader. Each call to Read hashes the bytes read before
// returning them.
func (hr *HashingReader) Read(p []byte) (int, error) {
	n, err := hr.r.Read(p)
	if n > 0 {
		// sha256.Write never returns an error.
		_, _ = hr.h.Write(p[:n])
	}
	return n, err
}

// Sum returns the hex-encoded SHA-256 digest of all data read so far. It
// may be called multiple times; each call returns the cumulative hash up to
// that point.
func (hr *HashingReader) Sum() string {
	return hex.EncodeToString(hr.h.Sum(nil))
}

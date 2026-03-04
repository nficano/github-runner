package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ComputeKey computes a deterministic cache key by hashing the contents of
// files matched by the given glob patterns. The resulting key is a hex-encoded
// SHA-256 digest prefixed with the joined path patterns for readability.
//
// paths is a list of filesystem path prefixes used to scope the cache (stored
// as metadata but not part of the hash). hashFiles is a list of glob patterns
// whose matched file contents are hashed to produce the unique portion of the
// key.
//
// Example:
//
//	key, err := ComputeKey(
//	    []string{"node_modules"},
//	    []string{"**/package-lock.json", "**/yarn.lock"},
//	)
func ComputeKey(paths []string, hashFiles []string) (string, error) {
	if len(hashFiles) == 0 {
		return "", fmt.Errorf("computing cache key: hashFiles must not be empty")
	}

	// Collect all files matching the glob patterns.
	var matched []string
	seen := make(map[string]bool)

	for _, pattern := range hashFiles {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("globbing pattern %q: %w", pattern, err)
		}
		for _, m := range matches {
			abs, err := filepath.Abs(m)
			if err != nil {
				return "", fmt.Errorf("resolving path %q: %w", m, err)
			}
			if !seen[abs] {
				seen[abs] = true
				matched = append(matched, abs)
			}
		}
	}

	if len(matched) == 0 {
		return "", fmt.Errorf("computing cache key: no files matched patterns %v", hashFiles)
	}

	// Sort for deterministic ordering.
	sort.Strings(matched)

	// Compute the aggregate SHA-256 hash of all matched file contents.
	h := sha256.New()
	for _, path := range matched {
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("opening file %q for hashing: %w", path, err)
		}

		// Write the filename as a separator so that two files with the same
		// content but different names produce different hashes.
		if _, err := fmt.Fprintf(h, "file:%s\n", path); err != nil {
			f.Close()
			return "", fmt.Errorf("writing filename to hash: %w", err)
		}

		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", fmt.Errorf("hashing file %q: %w", path, err)
		}
		f.Close()
	}

	digest := hex.EncodeToString(h.Sum(nil))

	// Build a human-readable prefix from the paths.
	prefix := strings.Join(paths, "-")
	if prefix == "" {
		return digest, nil
	}

	// Sanitise the prefix: replace path separators and special characters.
	prefix = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ' ' {
			return '-'
		}
		return r
	}, prefix)

	return prefix + "-" + digest, nil
}

// RestoreKey finds the best matching restore key for the given primary key
// and list of restore key prefixes. It returns the primary key if present in
// the restore keys, otherwise the longest prefix match. If no match is found,
// it returns an empty string.
//
// The matching strategy mirrors GitHub Actions cache behaviour:
//  1. Check for an exact match with the primary key.
//  2. For each restore key (in order), find the first prefix match.
//
// The restoreKeys are evaluated in the order provided; the first match wins.
func RestoreKey(key string, restoreKeys []string) string {
	if len(restoreKeys) == 0 {
		return ""
	}

	// Exact match takes priority.
	for _, rk := range restoreKeys {
		if rk == key {
			return rk
		}
	}

	// Prefix match: return the first restore key that is a prefix of the
	// primary key.
	for _, rk := range restoreKeys {
		if strings.HasPrefix(key, rk) {
			return rk
		}
	}

	return ""
}

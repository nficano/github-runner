// Package shell implements the shell-based executor that runs job steps
// directly on the host using os/exec.
package shell

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// envKeyRe matches valid POSIX environment variable names: uppercase/lowercase
// letters, digits, and underscores, starting with a letter or underscore.
var envKeyRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// SanitizeEnvKey reports whether key is a valid environment variable name.
// Valid names consist of ASCII letters, digits, and underscores and must
// start with a letter or underscore.
func SanitizeEnvKey(key string) bool {
	if key == "" {
		return false
	}
	return envKeyRe.MatchString(key)
}

// FilterEnv filters a list of environment variables (in KEY=VALUE format)
// using the provided allowlist and denylist. The rules are applied as follows:
//
//   - If allowlist is non-empty, only variables whose key appears in the
//     allowlist are kept.
//   - If denylist is non-empty, variables whose key appears in the denylist
//     are removed (denylist takes precedence over allowlist).
//   - Variables with invalid key names (per SanitizeEnvKey) are always dropped.
func FilterEnv(env []string, allowlist, denylist []string) []string {
	allow := toSet(allowlist)
	deny := toSet(denylist)

	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := parseEnvEntry(entry)
		if !ok {
			continue
		}
		if !SanitizeEnvKey(key) {
			continue
		}
		if _, denied := deny[key]; denied {
			continue
		}
		if len(allow) > 0 {
			if _, allowed := allow[key]; !allowed {
				continue
			}
		}
		out = append(out, entry)
	}
	return out
}

// MergeEnv merges base and override maps into a sorted slice of KEY=VALUE
// strings suitable for os/exec.Cmd.Env. Values in override take precedence
// over values in base. Keys that do not pass SanitizeEnvKey are skipped
// and an error is returned listing the invalid keys.
func MergeEnv(base, override map[string]string) ([]string, error) {
	merged := make(map[string]string, len(base)+len(override))
	var invalid []string

	for k, v := range base {
		if !SanitizeEnvKey(k) {
			invalid = append(invalid, k)
			continue
		}
		merged[k] = v
	}
	for k, v := range override {
		if !SanitizeEnvKey(k) {
			invalid = append(invalid, k)
			continue
		}
		merged[k] = v
	}

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)

	if len(invalid) > 0 {
		sort.Strings(invalid)
		return out, fmt.Errorf("invalid environment variable names: %s", strings.Join(invalid, ", "))
	}
	return out, nil
}

// parseEnvEntry splits a "KEY=VALUE" string. The ok return value is false
// if the entry does not contain an '=' separator.
func parseEnvEntry(entry string) (key, value string, ok bool) {
	idx := strings.IndexByte(entry, '=')
	if idx < 0 {
		return "", "", false
	}
	return entry[:idx], entry[idx+1:], true
}

// toSet converts a string slice into a set (map[string]struct{}) for O(1) lookups.
func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

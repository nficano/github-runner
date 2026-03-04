package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/mount"
)

// ParseVolumeMount parses a volume specification string in the format
// "host:container[:mode]" and returns a Docker mount.Mount. The mode
// component is optional and defaults to "rw" (read-write). Supported
// modes are "rw" (read-write) and "ro" (read-only).
//
// Examples:
//
//	"/host/path:/container/path"         -> bind mount, read-write
//	"/host/path:/container/path:ro"      -> bind mount, read-only
//	"/host/path:/container/path:rw"      -> bind mount, read-write
func ParseVolumeMount(spec string) (mount.Mount, error) {
	parts := strings.SplitN(spec, ":", 3)

	if len(parts) < 2 {
		return mount.Mount{}, fmt.Errorf("invalid volume spec %q: must be host:container[:mode]", spec)
	}

	source := parts[0]
	target := parts[1]

	if source == "" {
		return mount.Mount{}, fmt.Errorf("invalid volume spec %q: host path must not be empty", spec)
	}
	if target == "" {
		return mount.Mount{}, fmt.Errorf("invalid volume spec %q: container path must not be empty", spec)
	}

	readOnly := false
	if len(parts) == 3 {
		switch parts[2] {
		case "ro":
			readOnly = true
		case "rw":
			readOnly = false
		default:
			return mount.Mount{}, fmt.Errorf("invalid volume spec %q: mode must be 'ro' or 'rw', got %q", spec, parts[2])
		}
	}

	return mount.Mount{
		Type:     mount.TypeBind,
		Source:   source,
		Target:   target,
		ReadOnly: readOnly,
	}, nil
}

// ValidateVolumeMounts checks a list of mounts for common configuration
// errors:
//   - All source and target paths must be absolute.
//   - No two mounts may target the same container path.
//   - Certain sensitive host paths are rejected.
func ValidateVolumeMounts(mounts []mount.Mount) error {
	targets := make(map[string]struct{}, len(mounts))

	for _, m := range mounts {
		// Source path must be absolute.
		if !filepath.IsAbs(m.Source) {
			return fmt.Errorf("volume source %q must be an absolute path", m.Source)
		}

		// Target path must be absolute.
		if !filepath.IsAbs(m.Target) {
			return fmt.Errorf("volume target %q must be an absolute path", m.Target)
		}

		// Reject sensitive host paths to prevent container escapes.
		if isSensitivePath(m.Source) {
			return fmt.Errorf("volume source %q is a sensitive host path and cannot be mounted", m.Source)
		}

		// Check for duplicate targets.
		if _, exists := targets[m.Target]; exists {
			return fmt.Errorf("duplicate volume target %q", m.Target)
		}
		targets[m.Target] = struct{}{}
	}

	return nil
}

// sensitiveHostPaths is the set of host paths that should never be
// bind-mounted into a container.
var sensitiveHostPaths = []string{
	"/",
	"/dev",
	"/proc",
	"/sys",
	"/etc",
	"/var/run/docker.sock",
}

// isSensitivePath reports whether the given path (after cleaning) matches
// one of the well-known sensitive host paths.
func isSensitivePath(p string) bool {
	cleaned := filepath.Clean(p)
	for _, sensitive := range sensitiveHostPaths {
		if cleaned == sensitive {
			return true
		}
	}
	return false
}

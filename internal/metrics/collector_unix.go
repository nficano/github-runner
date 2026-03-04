//go:build unix

package metrics

import (
	"os"
	"path/filepath"
	"syscall"
)

// countFDsFallback counts open FDs by reading /dev/fd (macOS) or falls back
// to -1 if neither /proc/self/fd nor /dev/fd is available.
func countFDsFallback() (open int, max int) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return -1, maxFDsFallback()
	}
	// /dev/fd includes the directory FD opened by ReadDir itself; count
	// only entries that resolve to a real path.
	count := 0
	for _, e := range entries {
		if _, err := filepath.EvalSymlinks("/dev/fd/" + e.Name()); err == nil {
			count++
		}
	}
	return count, maxFDsFallback()
}

// maxFDsFallback returns the soft limit on open file descriptors via getrlimit.
func maxFDsFallback() int {
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return -1
	}
	return int(rlim.Cur)
}

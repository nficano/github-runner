//go:build !windows

package cache

import (
	"os"
	"syscall"
)

// flock acquires an exclusive advisory lock on the given file.
func flock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// funlock releases the advisory lock on the given file.
func funlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

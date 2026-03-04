//go:build windows

package cache

import (
	"os"
)

// flock acquires an exclusive lock on the given file. On Windows this is a
// no-op stub; a full implementation would use LockFileEx from the Windows API.
func flock(f *os.File) error {
	return nil
}

// funlock releases the lock on the given file. On Windows this is a no-op
// stub; a full implementation would use UnlockFileEx from the Windows API.
func funlock(f *os.File) error {
	return nil
}

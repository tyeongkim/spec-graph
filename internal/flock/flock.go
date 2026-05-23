package flock

import (
	"fmt"
	"os"
	"syscall"
)

// Lock acquires an exclusive advisory lock on the given file path.
// It creates the file if it doesn't exist with 0o644 permissions.
// Returns an unlock function that must be called to release the lock.
// The lock is held until the returned function is called or the process exits.
func Lock(path string) (func(), error) {
	// Create or open the lock file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// Acquire exclusive lock using syscall.Flock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	// Return unlock function that releases the lock and closes the file
	unlock := func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}

	return unlock, nil
}

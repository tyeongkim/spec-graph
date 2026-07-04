//go:build !windows

package flock

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// pollInterval is how often TryLock retries a contended lock while waiting for
// the deadline. It trades a small amount of latency for responsiveness under
// the low-contention agent workloads this tool targets.
const pollInterval = 50 * time.Millisecond

// Lock acquires an exclusive advisory lock on the given file path, blocking
// until the lock becomes available. It is retained for callers that genuinely
// want to wait indefinitely; most callers should prefer TryLock with a bounded
// timeout so a stuck holder surfaces as an error instead of a hang.
//
// It creates the file if it doesn't exist with 0o644 permissions and returns an
// unlock function that releases the lock and closes the file. The lock is held
// until the returned function is called or the process exits.
func Lock(path string) (func(), error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	return unlockFunc(f), nil
}

// TryLock attempts to acquire an exclusive advisory lock on the given file
// path without blocking indefinitely. It polls a non-blocking exclusive lock
// until it either succeeds or timeout elapses.
//
// A timeout <= 0 means a single non-blocking attempt with no retry. On timeout
// TryLock returns an error wrapping ErrTimeout so callers can distinguish
// contention (another process holds the lock) from other failures.
//
// It returns an unlock function that releases the lock and closes the file.
func TryLock(path string, timeout time.Duration) (func(), error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		lockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if lockErr == nil {
			return unlockFunc(f), nil
		}
		if lockErr != syscall.EWOULDBLOCK && lockErr != syscall.EAGAIN {
			_ = f.Close()
			return nil, fmt.Errorf("acquire lock: %w", lockErr)
		}

		// Contended: another process holds the lock. Retry until the deadline.
		if timeout <= 0 || !time.Now().Add(pollInterval).Before(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("acquire lock at %q: %w", path, ErrTimeout)
		}
		time.Sleep(pollInterval)
	}
}

func openLockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return f, nil
}

func unlockFunc(f *os.File) func() {
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
}

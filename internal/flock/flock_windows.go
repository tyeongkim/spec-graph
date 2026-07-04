//go:build windows

package flock

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

const pollInterval = 50 * time.Millisecond

// Lock acquires an exclusive lock on the given file path, blocking until the
// lock becomes available. See the Unix implementation for the shared contract;
// on Windows the lock is mandatory (enforced by the OS) rather than advisory.
func Lock(path string) (func(), error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}

	if err := lockFile(f, false); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	return unlockFunc(f), nil
}

// TryLock attempts to acquire an exclusive lock without blocking indefinitely,
// polling until it succeeds or timeout elapses. See the Unix implementation for
// the shared contract. On timeout it returns an error wrapping ErrTimeout.
func TryLock(path string, timeout time.Duration) (func(), error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		lockErr := lockFile(f, true)
		if lockErr == nil {
			return unlockFunc(f), nil
		}
		if lockErr != windows.ERROR_LOCK_VIOLATION && lockErr != windows.ERROR_IO_PENDING {
			_ = f.Close()
			return nil, fmt.Errorf("acquire lock: %w", lockErr)
		}

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

// lockFile locks the first byte of the file exclusively. When nonBlocking is
// true it fails immediately with ERROR_LOCK_VIOLATION if another process holds
// the lock; otherwise it blocks until the lock is granted. Locking a single
// byte is the standard idiom for a whole-file advisory-style lock via LockFileEx.
func lockFile(f *os.File, nonBlocking bool) error {
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)
	if nonBlocking {
		flags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	ol := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, ol)
}

func unlockFunc(f *os.File) func() {
	return func() {
		ol := new(windows.Overlapped)
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
		_ = f.Close()
	}
}

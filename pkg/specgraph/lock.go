package specgraph

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tyeongkim/spec-graph/internal/flock"
)

// defaultLockTimeout bounds how long an operation waits for the cross-process
// file lock before failing with a conflict, so a stuck holder surfaces as a
// clear error instead of an indefinite hang.
const defaultLockTimeout = 10 * time.Second

// lockManager mediates cross-process access to a spec-graph project through a
// single advisory file lock, while preserving in-process read concurrency.
//
// It reference-counts active operations within one process. The first operation
// (0->1) acquires the file lock and runs onAcquire to bring the index in sync
// with the on-disk source of truth; subsequent concurrent operations reuse the
// held lock without additional syscalls. The last operation to finish (1->0)
// releases the file lock.
//
// Because the file lock is held continuously while any operation is active, no
// other process can mutate the project during that window, so the index only
// needs to be refreshed once, at the 0->1 transition. onAcquire runs under the
// manager mutex with the reference count at zero, meaning no other in-process
// operation is active, so it may safely reopen or rebuild the index.
type lockManager struct {
	path      string
	timeout   time.Duration
	onAcquire func() error

	mu     sync.Mutex
	refs   int
	unlock func()
}

func newLockManager(path string, timeout time.Duration, onAcquire func() error) *lockManager {
	if timeout <= 0 {
		timeout = defaultLockTimeout
	}
	return &lockManager{path: path, timeout: timeout, onAcquire: onAcquire}
}

// acquire registers an active operation, taking the cross-process file lock and
// refreshing the index on the first concurrent entry. Every successful acquire
// must be paired with exactly one release.
func (m *lockManager) acquire() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refs > 0 {
		m.refs++
		return nil
	}

	unlock, err := flock.TryLock(m.path, m.timeout)
	if err != nil {
		return err
	}

	if m.onAcquire != nil {
		if err := m.onAcquire(); err != nil {
			unlock()
			return err
		}
	}

	m.unlock = unlock
	m.refs = 1
	return nil
}

// release ends an active operation, releasing the cross-process file lock once
// the last concurrent operation finishes.
func (m *lockManager) release() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refs == 0 {
		return
	}

	m.refs--
	if m.refs == 0 && m.unlock != nil {
		m.unlock()
		m.unlock = nil
	}
}

// withInitialLock runs fn once while holding the file lock, without registering
// a long-lived reference. It is used during Open to perform the initial index
// sync and then release the lock, so the process does not hold the lock for its
// entire lifetime.
func withInitialLock(path string, timeout time.Duration, fn func() error) error {
	if timeout <= 0 {
		timeout = defaultLockTimeout
	}
	unlock, err := flock.TryLock(path, timeout)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// writeLockedErr adapts writeLocked for operations that return only an error.
func writeLockedErr(e *Engine, fn func() error) error {
	_, err := writeLocked(e, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// conflictFromLock converts a flock timeout into a structured conflict Error and
// passes other errors through as runtime failures.
func conflictFromLock(root string, err error) error {
	if err == nil {
		return nil
	}
	if isLockTimeout(err) {
		return newError(CodeConflict, fmt.Sprintf("another spec-graph process holds the lock at %q", root), err)
	}
	return newError(CodeRuntime, fmt.Sprintf("acquire lock at %q", root), err)
}

func isLockTimeout(err error) bool {
	return errors.Is(err, flock.ErrTimeout)
}

package flock

import "errors"

// ErrTimeout is returned by TryLock when the lock could not be acquired before
// the timeout elapsed because another process holds it.
var ErrTimeout = errors.New("lock timeout: another process holds the lock")

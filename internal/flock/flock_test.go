//go:build !windows

package flock

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	defer unlock()

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
}

func TestLockBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock1, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	defer unlock1()

	var acquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		unlock2, err := Lock(lockPath)
		if err != nil {
			t.Errorf("second Lock failed: %v", err)
			return
		}
		acquired.Store(true)
		defer unlock2()
	}()

	time.Sleep(100 * time.Millisecond)
	if acquired.Load() {
		t.Error("second Lock acquired while first lock held (should block)")
	}

	unlock1()
	wg.Wait()

	if !acquired.Load() {
		t.Error("second Lock never acquired after first unlock")
	}
}

func TestLockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock1, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	unlock1()

	var acquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		unlock2, err := Lock(lockPath)
		if err != nil {
			t.Errorf("second Lock failed: %v", err)
			return
		}
		acquired.Store(true)
		defer unlock2()
	}()

	wg.Wait()

	if !acquired.Load() {
		t.Error("second Lock failed to acquire after first unlock")
	}
}

func TestConcurrentLockAttemptsAllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	const numGoroutines = 5
	var wg sync.WaitGroup
	var completed atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := Lock(lockPath)
			if err != nil {
				t.Errorf("Lock failed: %v", err)
				return
			}
			defer unlock()

			completed.Add(1)
		}()
	}

	wg.Wait()

	if got := completed.Load(); got != numGoroutines {
		t.Errorf("completed lock attempts = %d; want %d", got, numGoroutines)
	}
}

func TestTryLockAcquiresWhenFree(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock, err := TryLock(lockPath, time.Second)
	if err != nil {
		t.Fatalf("TryLock failed on a free lock: %v", err)
	}
	defer unlock()

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
}

func TestTryLockTimesOutWhenHeld(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock1, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	defer unlock1()

	start := time.Now()
	_, err = TryLock(lockPath, 150*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("TryLock returned too early (%v); expected it to wait near the timeout", elapsed)
	}
}

func TestTryLockZeroTimeoutFailsImmediately(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock1, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	defer unlock1()

	if _, err := TryLock(lockPath, 0); !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout on zero-timeout contended lock, got %v", err)
	}
}

func TestTryLockSucceedsAfterRelease(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	unlock1, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}

	var acquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		unlock2, tryErr := TryLock(lockPath, 5*time.Second)
		if tryErr != nil {
			t.Errorf("TryLock failed after release: %v", tryErr)
			return
		}
		acquired.Store(true)
		unlock2()
	}()

	time.Sleep(100 * time.Millisecond)
	if acquired.Load() {
		t.Error("TryLock acquired while first lock still held")
	}

	unlock1()
	wg.Wait()

	if !acquired.Load() {
		t.Error("TryLock never acquired after first unlock")
	}
}

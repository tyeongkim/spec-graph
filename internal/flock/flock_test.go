package flock

import (
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

func TestMultipleLocks(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	const numGoroutines = 5
	var wg sync.WaitGroup
	var counter int32
	var mu sync.Mutex

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

			mu.Lock()
			counter++
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)
		}()
	}

	wg.Wait()

	if counter != numGoroutines {
		t.Errorf("expected %d successful locks, got %d", numGoroutines, counter)
	}
}

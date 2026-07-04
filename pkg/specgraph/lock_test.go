package specgraph

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tyeongkim/spec-graph/internal/flock"
)

func newTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	return root
}

func TestOperationConflictsWithExternalLockHolder(t *testing.T) {
	root := newTestRoot(t)

	eng, err := Open(context.Background(), Options{Root: root, LockTimeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	unlock, err := flock.Lock(filepath.Join(root, ".lock"))
	if err != nil {
		t.Fatalf("simulate external holder: %v", err)
	}
	defer unlock()

	_, _, opErr := eng.ListEntities(context.Background(), ListEntitiesRequest{})
	if !IsConflict(opErr) {
		t.Fatalf("expected conflict while external process holds the lock, got %v", opErr)
	}
}

func TestOperationSucceedsAfterExternalLockReleased(t *testing.T) {
	root := newTestRoot(t)

	eng, err := Open(context.Background(), Options{Root: root, LockTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	unlock, err := flock.Lock(filepath.Join(root, ".lock"))
	if err != nil {
		t.Fatalf("simulate external holder: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		unlock()
	}()

	if _, _, opErr := eng.ListEntities(context.Background(), ListEntitiesRequest{}); opErr != nil {
		t.Fatalf("operation should have succeeded once the lock was released: %v", opErr)
	}
	wg.Wait()
}

func TestConcurrentInProcessOperations(t *testing.T) {
	root := newTestRoot(t)

	eng, err := Open(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	const workers = 8
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, _, opErr := eng.ListEntities(context.Background(), ListEntitiesRequest{}); opErr != nil {
					t.Errorf("concurrent read failed: %v", opErr)
					return
				}
			}
		}()
	}
	wg.Wait()

	if eng.lock.refs != 0 {
		t.Errorf("lock refcount = %d after all operations; want 0", eng.lock.refs)
	}
}

func TestLockRefcountReleasesAfterWrite(t *testing.T) {
	root := newTestRoot(t)

	eng, err := Open(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	if _, err := eng.CreateEntity(context.Background(), CreateEntityRequest{Type: "requirement", Title: "t"}); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	if eng.lock.refs != 0 {
		t.Errorf("lock refcount = %d after write; want 0", eng.lock.refs)
	}
	if eng.lock.unlock != nil {
		t.Error("lock should be fully released after a completed write")
	}
}

package specgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func openSnapshotEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".spec-graph")
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	return eng
}

// Read must not deadlock even though each Snapshot method reuses the locked path.
func TestReadDoesNotDeadlock(t *testing.T) {
	eng := openSnapshotEngine(t)
	ctx := context.Background()
	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "R"}); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- eng.Read(ctx, func(s *specgraph.Snapshot) error {
			for i := 0; i < 3; i++ {
				if _, _, err := s.ListEntities(specgraph.ListEntitiesRequest{}); err != nil {
					return err
				}
				if _, err := s.Validate(specgraph.ValidateRequest{}); err != nil {
					return err
				}
			}
			return nil
		})
	}()
	if err := <-done; err != nil {
		t.Fatalf("Read: %v", err)
	}
}

// A writer racing a Read must not interleave: every ListEntities inside one Read
// observes the same count.
func TestReadObservesOneRevision(t *testing.T) {
	eng := openSnapshotEngine(t)
	ctx := context.Background()
	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "R"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _ = eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type: "decision", Title: "D"})
		}
	}()

	for i := 0; i < 40; i++ {
		err := eng.Read(ctx, func(s *specgraph.Snapshot) error {
			_, first, err := s.ListEntities(specgraph.ListEntitiesRequest{})
			if err != nil {
				return err
			}
			for j := 0; j < 4; j++ {
				_, again, err := s.ListEntities(specgraph.ListEntitiesRequest{})
				if err != nil {
					return err
				}
				if again != first {
					t.Errorf("count changed inside one Read: %d then %d", first, again)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	wg.Wait()
}

package specgraph_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// TestEngineConcurrency proves RSK-002: the Engine is safe for concurrent use
// by multiple goroutines. It runs many concurrent readers and writers against a
// shared Engine. Run with -race to detect data races; the test asserts that no
// operation panics and that reads always observe a consistent graph.
func TestEngineConcurrency(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()

	const seedCount = 20
	for i := 0; i < seedCount; i++ {
		id := fmt.Sprintf("REQ-%03d", i+1)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    id,
			Title: "seed " + id,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	const workers = 16
	const iterations = 25

	var wg sync.WaitGroup
	errCh := make(chan error, workers*iterations)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				switch (worker + i) % 5 {
				case 0:
					if _, _, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{}); err != nil {
						errCh <- fmt.Errorf("list: %w", err)
					}
				case 1:
					id := fmt.Sprintf("REQ-%03d", (i%seedCount)+1)
					if _, err := eng.GetEntity(ctx, id); err != nil {
						errCh <- fmt.Errorf("get %s: %w", id, err)
					}
				case 2:
					if _, err := eng.Validate(ctx, specgraph.ValidateRequest{}); err != nil {
						errCh <- fmt.Errorf("validate: %w", err)
					}
				case 3:
					if _, err := eng.Export(ctx, specgraph.ExportRequest{Format: "json"}); err != nil {
						errCh <- fmt.Errorf("export: %w", err)
					}
				case 4:
					id := fmt.Sprintf("REQ-%03d", (i%seedCount)+1)
					newTitle := fmt.Sprintf("updated by %d-%d", worker, i)
					if _, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
						ID:    id,
						Title: &newTitle,
					}); err != nil {
						errCh <- fmt.Errorf("update %s: %w", id, err)
					}
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation failed: %v", err)
	}

	entities, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{})
	if err != nil {
		t.Fatalf("final list: %v", err)
	}
	if count != seedCount {
		t.Errorf("final count = %d, want %d", count, seedCount)
	}
	if len(entities) != seedCount {
		t.Errorf("final len = %d, want %d", len(entities), seedCount)
	}
}

// TestEngineConcurrentCreate proves that concurrent creates of distinct
// entities all succeed and are all observable afterward.
func TestEngineConcurrentCreate(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()

	const workers = 25

	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			id := fmt.Sprintf("REQ-%03d", worker+1)
			if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type:  "requirement",
				ID:    id,
				Title: "concurrent " + id,
			}); err != nil {
				errCh <- fmt.Errorf("create %s: %w", id, err)
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent create failed: %v", err)
	}

	_, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{})
	if err != nil {
		t.Fatalf("list after concurrent create: %v", err)
	}
	if count != workers {
		t.Errorf("count = %d, want %d", count, workers)
	}
}

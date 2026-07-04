package specgraph_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// newInitializedRoot returns the path to a fresh, isolated temp directory that
// looks like an initialized spec-graph project (i.e. it contains an entities/
// subdirectory, the minimum required for Open to succeed).
func newInitializedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	return root
}

// assertNotFound verifies that err is a *specgraph.Error carrying CodeNotFound
// and that the IsNotFound predicate agrees.
func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var sgErr *specgraph.Error
	if !errors.As(err, &sgErr) {
		t.Fatalf("expected error to be *specgraph.Error, got %T: %v", err, err)
	}
	if sgErr.Code != specgraph.CodeNotFound {
		t.Errorf("got error code %q; want %q", sgErr.Code, specgraph.CodeNotFound)
	}
	if !specgraph.IsNotFound(err) {
		t.Error("IsNotFound returned false; want true")
	}
}

// TestEngineOpenClose proves ACT-001: the Engine Open/Close lifecycle works.
func TestEngineOpenClose(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		root := newInitializedRoot(t)

		eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("Open returned unexpected error: %v", err)
		}
		if eng == nil {
			t.Fatal("Open returned nil Engine without an error")
		}

		// A successful Open must have created the SQLite index (graph.db).
		dbPath := filepath.Join(root, "graph.db")
		if _, statErr := os.Stat(dbPath); statErr != nil {
			t.Errorf("expected graph.db to exist after Open: %v", statErr)
		}

		if closeErr := eng.Close(); closeErr != nil {
			t.Errorf("Close returned unexpected error: %v", closeErr)
		}
	})

	t.Run("not initialized - missing dir", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "does-not-exist")

		eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: missing})
		if eng != nil {
			_ = eng.Close()
			t.Fatal("Open returned a non-nil Engine for a missing directory")
		}
		assertNotFound(t, err)
	})

	t.Run("not initialized - no entities dir", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if eng != nil {
			_ = eng.Close()
			t.Fatal("Open returned a non-nil Engine for a dir without entities/")
		}
		assertNotFound(t, err)
	})

	t.Run("double close", func(t *testing.T) {
		t.Parallel()

		root := newInitializedRoot(t)

		eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("Open returned unexpected error: %v", err)
		}

		if closeErr := eng.Close(); closeErr != nil {
			t.Errorf("first Close returned unexpected error: %v", closeErr)
		}

		// The second Close must be safe: it must not panic. Any returned error
		// is tolerated, but a panic is a hard failure.
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("second Close panicked: %v", r)
				}
			}()
			_ = eng.Close()
		}()
	})

	t.Run("concurrent opens coexist", func(t *testing.T) {
		t.Parallel()

		root := newInitializedRoot(t)

		first, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("first Open returned unexpected error: %v", err)
		}
		defer first.Close()

		second, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("second Open must succeed now that the lock is per-operation, got: %v", err)
		}
		defer second.Close()

		if _, _, err := first.ListEntities(context.Background(), specgraph.ListEntitiesRequest{}); err != nil {
			t.Errorf("first engine operation failed: %v", err)
		}
		if _, _, err := second.ListEntities(context.Background(), specgraph.ListEntitiesRequest{}); err != nil {
			t.Errorf("second engine operation failed: %v", err)
		}
	})

	t.Run("cross-process write is visible to a second engine", func(t *testing.T) {
		t.Parallel()

		root := newInitializedRoot(t)

		writer, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("writer Open: %v", err)
		}
		defer writer.Close()

		reader, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("reader Open: %v", err)
		}
		defer reader.Close()

		created, err := writer.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
			Type:  "requirement",
			Title: "Cross-process visible",
		})
		if err != nil {
			t.Fatalf("writer CreateEntity: %v", err)
		}

		got, err := reader.GetEntity(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("reader GetEntity after cross-engine write: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("reader saw entity %q; want %q", got.ID, created.ID)
		}
	})
}

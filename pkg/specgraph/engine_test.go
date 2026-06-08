package specgraph_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	t.Run("lock exclusivity", func(t *testing.T) {
		t.Parallel()

		root := newInitializedRoot(t)

		first, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
		if err != nil {
			t.Fatalf("first Open returned unexpected error: %v", err)
		}

		// openResult carries the outcome of the second, contending Open.
		type openResult struct {
			eng *specgraph.Engine
			err error
		}
		done := make(chan openResult, 1)

		go func() {
			eng, openErr := specgraph.Open(context.Background(), specgraph.Options{Root: root})
			done <- openResult{eng: eng, err: openErr}
		}()

		// While the first Engine holds the lock, the second Open must not
		// complete: the exclusive lock should either block it or keep it from
		// returning a usable Engine.
		select {
		case res := <-done:
			if res.err == nil {
				if res.eng != nil {
					_ = res.eng.Close()
				}
				_ = first.Close()
				t.Fatal("second Open succeeded while the first Engine held the lock")
			}
			// A non-nil error here (rather than blocking) is also an acceptable
			// way to enforce exclusivity, so the test continues.
		case <-time.After(200 * time.Millisecond):
			// Expected: the second Open is blocked on the exclusive lock.
		}

		// Releasing the first lock must let the contending Open proceed.
		if closeErr := first.Close(); closeErr != nil {
			t.Errorf("Close of first Engine returned unexpected error: %v", closeErr)
		}

		select {
		case res := <-done:
			if res.err != nil {
				t.Fatalf("second Open failed after first was closed: %v", res.err)
			}
			if res.eng == nil {
				t.Fatal("second Open returned nil Engine without an error")
			}
			if closeErr := res.eng.Close(); closeErr != nil {
				t.Errorf("Close of second Engine returned unexpected error: %v", closeErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second Open did not complete after the first Engine was closed")
		}
	})
}

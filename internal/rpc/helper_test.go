package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// newTestEngine opens an Engine rooted at a fresh, initialized temp directory
// and registers a cleanup that closes it.
func newTestEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

package specgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// openSandboxedEngine opens an Engine whose graph root is nested inside a
// separate sandbox directory, so a test can assert that no file escapes the
// graph root into the surrounding sandbox.
func openSandboxedEngine(t *testing.T) (*specgraph.Engine, string, string) {
	t.Helper()

	sandbox := t.TempDir()
	root := filepath.Join(sandbox, "project", ".spec-graph")
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}

	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	return eng, sandbox, root
}

func TestBootstrapImport_RejectsPathTraversalCandidates(t *testing.T) {
	tests := []struct {
		name          string
		candidateID   string
		candidateType string
	}{
		{"traversal in type", "proof", "../../.."},
		{"traversal in id", "../../../proof", "requirement"},
		{"traversal in both", "../proof", "../.."},
		{"separator in type", "REQ-001", "requirement/nested"},
		{"unknown type", "REQ-001", "not-a-type"},
		{"malformed id", "not-an-id", "requirement"},
		{"id prefix mismatch", "DEC-001", "requirement"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng, sandbox, root := openSandboxedEngine(t)

			result, err := eng.BootstrapImport(context.Background(), specgraph.BootstrapImportRequest{
				Entities: []specgraph.BootstrapCandidate{{
					ID:         tc.candidateID,
					Type:       tc.candidateType,
					Title:      "traversal probe",
					Confidence: 0.9,
				}},
			})
			if err != nil {
				t.Fatalf("BootstrapImport returned a transport error: %v", err)
			}

			if len(result.Created) != 0 {
				t.Errorf("Created = %v; want no created entities", result.Created)
			}
			if len(result.Errors) == 0 {
				t.Error("Errors is empty; want the invalid candidate reported")
			}

			assertGraphRootConfined(t, sandbox, root)
		})
	}
}

func TestBootstrapImport_AcceptsValidCandidate(t *testing.T) {
	eng, sandbox, root := openSandboxedEngine(t)

	result, err := eng.BootstrapImport(context.Background(), specgraph.BootstrapImportRequest{
		Entities: []specgraph.BootstrapCandidate{{
			ID:         "REQ-001",
			Type:       "requirement",
			Title:      "User authentication",
			Confidence: 0.9,
		}},
	})
	if err != nil {
		t.Fatalf("BootstrapImport: %v", err)
	}

	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v; want none", result.Errors)
	}
	if len(result.Created) != 1 || result.Created[0] != "REQ-001" {
		t.Fatalf("Created = %v; want [REQ-001]", result.Created)
	}

	written := filepath.Join(root, "entities", "requirement", "REQ-001.toml")
	if _, statErr := os.Stat(written); statErr != nil {
		t.Errorf("expected entity file at %s: %v", written, statErr)
	}

	assertGraphRootConfined(t, sandbox, root)
}

func assertGraphRootConfined(t *testing.T, sandbox, graphRoot string) {
	t.Helper()

	err := filepath.WalkDir(sandbox, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(graphRoot, path)
		if relErr != nil {
			return relErr
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			t.Errorf("file written outside the graph root: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk sandbox: %v", err)
	}
}

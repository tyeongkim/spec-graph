package specgraph_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestExport(t *testing.T) {
	t.Parallel()

	// seedExport creates API-001 implements REQ-001 plus an isolated DEC-001,
	// giving the exporter both connected and unconnected nodes.
	seedExport := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "decision", ID: "DEC-001", Title: "Isolated"}); err != nil {
			t.Fatalf("create DEC-001: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		return eng, ctx
	}

	t.Run("invalid format", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.Export(ctx, specgraph.ExportRequest{Format: "yaml"})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("dot format", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedExport(t)

		res, err := eng.Export(ctx, specgraph.ExportRequest{Format: "dot"})
		if err != nil {
			t.Fatalf("Export dot: %v", err)
		}
		if res.Format != "dot" {
			t.Errorf("Format = %q, want %q", res.Format, "dot")
		}
		if !strings.Contains(res.Data, "digraph") {
			t.Errorf("dot output missing 'digraph': %q", res.Data)
		}
	})

	t.Run("mermaid format", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedExport(t)

		res, err := eng.Export(ctx, specgraph.ExportRequest{Format: "mermaid"})
		if err != nil {
			t.Fatalf("Export mermaid: %v", err)
		}
		if res.Format != "mermaid" {
			t.Errorf("Format = %q, want %q", res.Format, "mermaid")
		}
		if !strings.Contains(res.Data, "graph") && !strings.Contains(res.Data, "flowchart") {
			t.Errorf("mermaid output missing graph/flowchart directive: %q", res.Data)
		}
	})

	t.Run("json format", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedExport(t)

		res, err := eng.Export(ctx, specgraph.ExportRequest{Format: "json"})
		if err != nil {
			t.Fatalf("Export json: %v", err)
		}
		if res.Format != "json" {
			t.Errorf("Format = %q, want %q", res.Format, "json")
		}
		var parsed map[string]json.RawMessage
		if err := json.Unmarshal([]byte(res.Data), &parsed); err != nil {
			t.Fatalf("json output is not valid JSON object: %v", err)
		}
	})

	t.Run("center and depth exports subgraph only", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedExport(t)

		// Centering on API-001 at depth 1 includes API-001 and REQ-001 but
		// excludes the unconnected DEC-001.
		res, err := eng.Export(ctx, specgraph.ExportRequest{Format: "json", Center: "API-001", Depth: 1})
		if err != nil {
			t.Fatalf("Export center: %v", err)
		}
		if strings.Contains(res.Data, "DEC-001") {
			t.Errorf("subgraph should not contain DEC-001: %q", res.Data)
		}
		if !strings.Contains(res.Data, "API-001") || !strings.Contains(res.Data, "REQ-001") {
			t.Errorf("subgraph should contain API-001 and REQ-001: %q", res.Data)
		}
	})
}

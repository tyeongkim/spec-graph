package specgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestPhaseGateReportsStructuralExecEdgesWithoutMutation(t *testing.T) {
	root := newInitializedRoot(t)
	ctx := context.Background()
	eng, err := specgraph.Open(ctx, specgraph.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	for _, req := range []specgraph.CreateEntityRequest{
		{Type: "phase", ID: "PHS-001", Title: "Phase", Status: "active"},
		{Type: "requirement", ID: "REQ-001", Title: "Requirement"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "entities", "phase", "PHS-001.toml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := append(original, []byte("\n[[relations]]\nto = \"REQ-001\"\ntype = \"belongs_to\"\n")...)
	if err := os.WriteFile(path, invalid, 0644); err != nil {
		t.Fatal(err)
	}
	report, err := eng.EvaluatePhaseGate(ctx, "PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range report.BlockingIssues {
		if issue.Check == "invalid_exec_edges" {
			found = true
		}
	}
	if !found || !report.StructuralBlocked {
		t.Fatalf("structural exec edge not blocking: %+v", report)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(invalid) {
		t.Fatal("gate changed phase file")
	}
	resolved := "resolved"
	update, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "PHS-001", Status: &resolved})
	if err != nil {
		t.Fatal(err)
	}
	if update.Outcome != specgraph.UpdateOutcomeBlocked || update.GateReport == nil || !update.GateReport.StructuralBlocked {
		t.Fatalf("transition disagrees with preflight: %+v", update)
	}
}

package specgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestTaskResolveUsesCandidateEvidence(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	evidence := writeTaskEvidence(t, root, "candidate-evidence.txt")

	contract := validTaskContract()
	contract.QA[0].Evidence = evidence
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "TSK-001", Status: &resolved, Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("resolve task with candidate evidence: %v", err)
	}
	if result.Entity.Status != model.EntityStatusResolved {
		t.Fatalf("status = %q; want resolved", result.Entity.Status)
	}
}

func TestTaskPhasePlanCompletionFlow(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	evidence := writeTaskEvidence(t, root, "completion-evidence.txt")

	contract := validTaskContract()
	contract.QA[0].Evidence = evidence
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	for _, update := range []specgraph.UpdateEntityRequest{
		{ID: "TSK-001", Status: &resolved, Metadata: &metadata},
		{ID: "PHS-001", Status: &resolved},
		{ID: "PLN-001", Status: &resolved},
	} {
		result, err := engine.UpdateEntity(context.Background(), update)
		if err != nil {
			t.Fatalf("resolve %s: %v", update.ID, err)
		}
		if result.GateReport != nil && result.GateReport.Blocked {
			t.Fatalf("resolve %s blocked: %+v", update.ID, result.GateReport.BlockingIssues)
		}
	}

	for _, id := range []string{"TSK-001", "PHS-001", "PLN-001"} {
		entity, err := engine.GetEntity(context.Background(), id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if entity.Status != model.EntityStatusResolved {
			t.Errorf("%s status = %q; want resolved", id, entity.Status)
		}
	}
}

func TestForceCompletionReturnsGateReport(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	resolved := string(model.EntityStatusResolved)

	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "PHS-001", Status: &resolved, Force: true, Reason: "Accept unfinished task",
	})
	if err != nil {
		t.Fatalf("force phase completion: %v", err)
	}
	if result.Entity.Status != model.EntityStatusResolved {
		t.Fatalf("status = %q; want resolved", result.Entity.Status)
	}
	if result.GateReport == nil || !result.GateReport.Blocked {
		t.Fatal("forced completion did not return its blocking gate report")
	}
	if len(result.GateReport.BlockingIssues) == 0 {
		t.Fatal("forced completion returned an empty gate report")
	}
}

func TestTaskResolveRejectsInvalidEvidencePaths(t *testing.T) {
	tests := []struct {
		name     string
		evidence func(*testing.T, string) string
	}{
		{name: "missing file", evidence: func(_ *testing.T, _ string) string { return "missing.txt" }},
		{name: "directory", evidence: func(t *testing.T, root string) string {
			dir := filepath.Join(filepath.Dir(root), "evidence-dir")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatalf("create evidence directory: %v", err)
			}
			return filepath.Base(dir)
		}},
		{name: "outside root", evidence: func(t *testing.T, root string) string {
			outside := filepath.Join(filepath.Dir(filepath.Dir(root)), "outside-evidence.txt")
			if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
				t.Fatalf("write outside evidence: %v", err)
			}
			return filepath.Join("..", filepath.Base(outside))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, engine := openTaskTestEngine(t)
			setupTaskLifecycle(t, engine, "TSK-001", true)
			path := taskEntityPath(root, "TSK-001")
			before := readTaskBytes(t, path)
			contract := validTaskContract()
			contract.QA[0].Evidence = test.evidence(t, root)
			metadata := marshalTaskContract(t, contract)
			resolved := string(model.EntityStatusResolved)

			result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
				ID: "TSK-001", Status: &resolved, Metadata: &metadata,
			})
			if err != nil {
				t.Fatalf("resolve task: %v", err)
			}
			if result.GateReport == nil || !result.GateReport.Blocked {
				t.Fatal("invalid evidence path did not block resolution")
			}
			if after := readTaskBytes(t, path); !reflect.DeepEqual(after, before) {
				t.Fatal("task TOML changed after invalid evidence was rejected")
			}
		})
	}
}

func TestBlockedTransitionIsAtomic(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", false)
	path := taskEntityPath(root, "TSK-001")
	before := readTaskBytes(t, path)
	contract := validTaskContract()
	contract.QA[0].Evidence = writeTaskEvidence(t, root, "atomic-evidence.txt")
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)

	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "TSK-001", Status: &resolved, Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("blocked update: %v", err)
	}
	if result.GateReport == nil || !result.GateReport.Blocked {
		t.Fatal("task without matching delivers was not blocked")
	}
	if after := readTaskBytes(t, path); !reflect.DeepEqual(after, before) {
		t.Fatal("blocked transition changed task TOML")
	}
}

func TestForceCannotBypassLifecycle(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	if _, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
		Type: "phase", ID: "PHS-001", Title: "Resolved phase", Status: "resolved",
	}); err != nil {
		t.Fatalf("create resolved phase: %v", err)
	}
	active := string(model.EntityStatusActive)
	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "PHS-001", Status: &active, Force: true, Reason: "Attempt reopen",
	})
	if err != nil {
		t.Fatalf("forced lifecycle update: %v", err)
	}
	if result.GateReport == nil || !result.GateReport.StructuralBlocked {
		t.Fatal("force bypassed a terminal lifecycle transition")
	}
	entity, getErr := engine.GetEntity(context.Background(), "PHS-001")
	if getErr != nil {
		t.Fatalf("get phase: %v", getErr)
	}
	if entity.Status != model.EntityStatusResolved {
		t.Fatalf("status = %q; want resolved", entity.Status)
	}
}

func setupTaskLifecycle(t *testing.T, engine *specgraph.Engine, taskID string, withDelivery bool) {
	t.Helper()
	ctx := context.Background()
	for _, request := range []specgraph.CreateEntityRequest{
		{Type: "plan", ID: "PLN-001", Title: "Plan", Status: "active"},
		{Type: "phase", ID: "PHS-001", Title: "Phase", Status: "active"},
	} {
		if _, err := engine.CreateEntity(ctx, request); err != nil {
			t.Fatalf("create %s: %v", request.ID, err)
		}
	}
	createTask(t, engine, taskID)
	for _, request := range []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: taskID, To: "PHS-001", Type: "belongs_to"},
	} {
		if _, err := engine.AddRelation(ctx, request); err != nil {
			t.Fatalf("add %s relation: %v", request.Type, err)
		}
	}
	if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Requirement", Status: "active"}); err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{From: taskID, To: "REQ-001", Type: "covers"}); err != nil {
		t.Fatalf("add covers: %v", err)
	}
	if withDelivery {
		if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{From: taskID, To: "REQ-001", Type: "delivers"}); err != nil {
			t.Fatalf("add delivers: %v", err)
		}
	}
	active := string(model.EntityStatusActive)
	result, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: taskID, Status: &active})
	if err != nil {
		t.Fatalf("activate task: %v", err)
	}
	if result.GateReport != nil && result.GateReport.Blocked {
		t.Fatalf("activate task blocked: %+v", result.GateReport.BlockingIssues)
	}
}

func writeTaskEvidence(t *testing.T, specRoot, name string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(specRoot), name)
	if err := os.WriteFile(path, []byte("verified"), 0o600); err != nil {
		t.Fatalf("write task evidence: %v", err)
	}
	return name
}

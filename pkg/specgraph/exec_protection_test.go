package specgraph_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestExecTransitionMatrixAppliesToEveryExecType(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		id   string
		from string
		to   string
	}{
		{name: "active plan to draft", typ: "plan", id: "PLN-001", from: "active", to: "draft"},
		{name: "resolved plan to active", typ: "plan", id: "PLN-001", from: "resolved", to: "active"},
		{name: "active phase to draft", typ: "phase", id: "PHS-001", from: "active", to: "draft"},
		{name: "resolved phase to deprecated", typ: "phase", id: "PHS-001", from: "resolved", to: "deprecated"},
		{name: "draft phase to resolved", typ: "phase", id: "PHS-001", from: "draft", to: "resolved"},
		{name: "deprecated change to active", typ: "change", id: "CHG-001", from: "deprecated", to: "active"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, engine := openTaskTestEngine(t)
			ctx := context.Background()
			if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type: test.typ, ID: test.id, Title: "Fixture", Status: test.from,
			}); err != nil {
				t.Fatalf("create: %v", err)
			}
			_, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
				ID: test.id, Status: &test.to, Force: true, Reason: "Attempt invalid transition",
			})
			if !specgraph.IsInvalidInput(err) {
				t.Fatalf("error = %v; want invalid input", err)
			}
			stored, err := engine.GetEntity(ctx, test.id)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if string(stored.Status) != test.from {
				t.Fatalf("status = %q; want unchanged %q", stored.Status, test.from)
			}
		})
	}
}

func TestActiveTaskContractIsFrozen(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	ctx := context.Background()
	before, err := engine.GetEntity(ctx, "TSK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	contract := validTaskContract()
	contract.Instructions = []string{"Do something else entirely."}
	metadata := marshalTaskContract(t, contract)
	newTitle := "Renamed task"

	result, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
		ID: "TSK-001", Title: &newTitle, Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeBlocked {
		t.Fatalf("outcome = %q; want blocked", result.Outcome)
	}
	if result.GateReport == nil || result.GateReport.StructuralBlocked {
		t.Fatalf("gate report = %+v; want forceable frozen_fields block", result.GateReport)
	}
	fields := map[string]bool{}
	for _, issue := range result.GateReport.BlockingIssues {
		if issue.Check != "frozen_fields" {
			t.Errorf("unexpected check %q", issue.Check)
		}
		fields[issue.Message] = true
	}
	if len(fields) != 2 {
		t.Fatalf("blocking issues = %+v; want one for title and one for metadata", result.GateReport.BlockingIssues)
	}

	after, err := engine.GetEntity(ctx, "TSK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if after.Title != before.Title || string(after.Metadata) != string(before.Metadata) {
		t.Fatal("blocked update changed the stored task")
	}
}

func TestActiveTaskContractForceRecordsAmendment(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	ctx := context.Background()

	contract := validTaskContract()
	contract.Acceptance = []string{"A narrower acceptance."}
	metadata := marshalTaskContract(t, contract)

	result, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
		ID: "TSK-001", Metadata: &metadata, Force: true, Reason: "Scope clarified with stakeholder",
	})
	if err != nil {
		t.Fatalf("forced update: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeAppliedWithForce {
		t.Fatalf("outcome = %q; want applied_with_force", result.Outcome)
	}
	if !result.Entity.AmendmentForced || result.Entity.AmendmentReason != "Scope clarified with stakeholder" {
		t.Fatalf("amendment audit = forced:%t reason:%q", result.Entity.AmendmentForced, result.Entity.AmendmentReason)
	}
	if result.Entity.CompletionForced {
		t.Fatal("amendment must not be recorded as a forced completion")
	}

	stored, err := engine.GetEntity(ctx, "TSK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !stored.AmendmentForced || stored.AmendmentReason != "Scope clarified with stakeholder" {
		t.Fatalf("stored amendment audit = forced:%t reason:%q", stored.AmendmentForced, stored.AmendmentReason)
	}
}

func TestActiveTaskContractForceRequiresReason(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)

	contract := validTaskContract()
	contract.Order = 7
	metadata := marshalTaskContract(t, contract)
	_, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "TSK-001", Metadata: &metadata, Force: true,
	})
	if !specgraph.IsInvalidInput(err) {
		t.Fatalf("error = %v; want invalid input", err)
	}
}

func TestActiveTaskMayStillRecordEvidence(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)

	contract := validTaskContract()
	contract.QA[0].Evidence = writeTaskEvidence(t, root, "evidence-only.txt")
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "TSK-001", Status: &resolved, Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeApplied {
		t.Fatalf("outcome = %q; want applied; report=%+v", result.Outcome, result.GateReport)
	}
	if result.Entity.AmendmentForced {
		t.Fatal("filling evidence must not count as an amendment")
	}
}

func TestActivePhaseTitleAndMetadataAreFrozen(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	ctx := context.Background()

	metadata := json.RawMessage(`{"goal":"Changed goal","order":1}`)
	result, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "PHS-001", Metadata: &metadata})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeBlocked {
		t.Fatalf("outcome = %q; want blocked", result.Outcome)
	}

	description := "A description is not frozen."
	result, err = engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "PHS-001", Description: &description})
	if err != nil {
		t.Fatalf("update description: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeBlocked {
		t.Fatalf("description outcome = %q; want blocked", result.Outcome)
	}
}

func TestDraftExecEntityFieldsAreFree(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	createTask(t, engine, "TSK-001")

	contract := validTaskContract()
	contract.Instructions = []string{"Revised while still in draft."}
	metadata := marshalTaskContract(t, contract)
	title := "Revised draft task"
	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "TSK-001", Title: &title, Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if result.Outcome != specgraph.UpdateOutcomeApplied || result.Entity.AmendmentForced {
		t.Fatalf("result = %+v; want plain apply", result)
	}
}

func TestRelationGuardsOnStartedExecEntities(t *testing.T) {
	type attempt struct {
		name string
		add  *specgraph.AddRelationRequest
		del  *specgraph.DeleteRelationRequest
	}
	tests := []attempt{
		{name: "remove belongs_to from active task", del: &specgraph.DeleteRelationRequest{From: "TSK-001", To: "PHS-001", Type: "belongs_to"}},
		{name: "remove belongs_to from active phase", del: &specgraph.DeleteRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"}},
		{name: "attach new task to active phase", add: &specgraph.AddRelationRequest{From: "TSK-002", To: "PHS-001", Type: "belongs_to"}},
		{name: "add prerequisite to active task", add: &specgraph.AddRelationRequest{From: "TSK-001", To: "TSK-002", Type: "task_depends_on"}},
		{name: "add covers to active task", add: &specgraph.AddRelationRequest{From: "TSK-001", To: "REQ-002", Type: "covers"}},
		{name: "remove covers from active task", del: &specgraph.DeleteRelationRequest{From: "TSK-001", To: "REQ-001", Type: "covers"}},
		{name: "precedes into active phase", add: &specgraph.AddRelationRequest{From: "PHS-002", To: "PHS-001", Type: "precedes"}},
		{name: "precedes out of active phase", add: &specgraph.AddRelationRequest{From: "PHS-001", To: "PHS-002", Type: "precedes"}},
		{name: "blocks into active phase", add: &specgraph.AddRelationRequest{From: "PHS-002", To: "PHS-001", Type: "blocks"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, engine := openTaskTestEngine(t)
			setupTaskLifecycle(t, engine, "TSK-001", true)
			ctx := context.Background()
			createTask(t, engine, "TSK-002")
			for _, request := range []specgraph.CreateEntityRequest{
				{Type: "phase", ID: "PHS-002", Title: "Draft phase"},
				{Type: "requirement", ID: "REQ-002", Title: "Another requirement", Status: "active"},
			} {
				if _, err := engine.CreateEntity(ctx, request); err != nil {
					t.Fatalf("create %s: %v", request.ID, err)
				}
			}

			var err error
			if test.add != nil {
				_, err = engine.AddRelation(ctx, *test.add)
			} else {
				err = engine.DeleteRelation(ctx, *test.del)
			}
			if !specgraph.IsInvalidInput(err) {
				t.Fatalf("error = %v; want invalid input", err)
			}
		})
	}
}

func TestActiveTaskMayStillAddDelivers(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", false)
	if _, err := engine.AddRelation(context.Background(), specgraph.AddRelationRequest{
		From: "TSK-001", To: "REQ-001", Type: "delivers",
	}); err != nil {
		t.Fatalf("add delivers to active task: %v", err)
	}
}

func TestResolvedTaskDeliveryRecordIsFixed(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	ctx := context.Background()
	contract := validTaskContract()
	contract.QA[0].Evidence = writeTaskEvidence(t, root, "fixed-delivery.txt")
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	if _, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "TSK-001", Status: &resolved, Metadata: &metadata}); err != nil {
		t.Fatalf("resolve task: %v", err)
	}

	if err := engine.DeleteRelation(ctx, specgraph.DeleteRelationRequest{From: "TSK-001", To: "REQ-001", Type: "delivers"}); !specgraph.IsInvalidInput(err) {
		t.Fatalf("delete delivers error = %v; want invalid input", err)
	}
	if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-002", Title: "Late requirement", Status: "active"}); err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{From: "TSK-001", To: "REQ-002", Type: "delivers"}); !specgraph.IsInvalidInput(err) {
		t.Fatalf("add delivers error = %v; want invalid input", err)
	}
}

func TestDraftPhaseMayBeAppendedAfterActivePlanStarts(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	ctx := context.Background()
	if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-002", Title: "Next phase"}); err != nil {
		t.Fatalf("create phase: %v", err)
	}
	if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-002", To: "PLN-001", Type: "belongs_to"}); err != nil {
		t.Fatalf("attach draft phase to active plan: %v", err)
	}
}

func TestStartedExecEntitiesCannotBeDeleted(t *testing.T) {
	tests := []struct {
		typ, id, status string
	}{
		{"change", "CHG-001", "active"},
		{"plan", "PLN-001", "resolved"},
		{"phase", "PHS-001", "deprecated"},
	}
	for _, test := range tests {
		t.Run(test.typ+" "+test.status, func(t *testing.T) {
			_, engine := openTaskTestEngine(t)
			ctx := context.Background()
			if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type: test.typ, ID: test.id, Title: "Fixture", Status: test.status,
			}); err != nil {
				t.Fatalf("create: %v", err)
			}
			if err := engine.DeleteEntity(ctx, test.id); !specgraph.IsInvalidInput(err) {
				t.Fatalf("delete error = %v; want invalid input", err)
			}
			if _, err := engine.GetEntity(ctx, test.id); err != nil {
				t.Fatalf("entity was removed: %v", err)
			}
		})
	}
}

func TestDraftExecEntityCanBeDeleted(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	ctx := context.Background()
	if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "change", ID: "CHG-001", Title: "Draft change"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := engine.DeleteEntity(ctx, "CHG-001"); err != nil {
		t.Fatalf("delete draft change: %v", err)
	}
}

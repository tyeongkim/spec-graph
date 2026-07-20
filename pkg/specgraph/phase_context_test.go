package specgraph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestPhaseContextCompleteDeterministicResult(t *testing.T) {
	engine := openTestEngine(t)
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{
		Type: "plan", ID: "PLN-001", Title: "Delivery plan", Description: "Complete parent plan.",
		Metadata: json.RawMessage(`{"owner":"platform"}`),
	})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{
		Type: "phase", ID: "PHS-001", Title: "Context phase", Description: "Complete child phase.",
		Metadata: json.RawMessage(`{"goal":"Expose phase context","order":1,"exit_criteria":["Context is deterministic"]}`),
	})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "First requirement", Description: "First covered architecture entity.",
		Metadata: json.RawMessage(`{"priority":"must","kind":"functional","owner":"platform"}`),
	})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-002", Title: "Second requirement", Description: "Second covered architecture entity.",
		Metadata: json.RawMessage(`{"priority":"should","kind":"non_functional","owner":"quality"}`),
	})
	createPhaseContextTask(t, engine, "TSK-001", 2, "Implement the engine result.")
	createPhaseContextTask(t, engine, "TSK-002", 1, "Preserve complete entities.")
	createPhaseContextTask(t, engine, "TSK-003", 1, "Consume the engine result.")

	for _, relation := range []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: "TSK-001", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-002", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-003", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-003", To: "TSK-001", Type: "task_depends_on"},
		{From: "TSK-001", To: "REQ-001", Type: "covers"},
		{From: "TSK-002", To: "REQ-002", Type: "covers"},
		{From: "TSK-002", To: "REQ-002", Type: "delivers"},
		{From: "TSK-003", To: "REQ-001", Type: "covers"},
	} {
		addPhaseContextRelation(t, engine, relation)
	}

	result, err := engine.PhaseContext("PHS-001")
	if err != nil {
		t.Fatalf("PhaseContext: %v", err)
	}

	if result.Plan.ID != "PLN-001" || result.Plan.Description != "Complete parent plan." || len(result.Plan.Metadata) == 0 {
		t.Errorf("plan lost full entity data: %+v", result.Plan)
	}
	if result.Phase.ID != "PHS-001" || result.Phase.Description != "Complete child phase." || len(result.Phase.Metadata) == 0 {
		t.Errorf("phase lost full entity data: %+v", result.Phase)
	}
	if got := taskContextIDs(result.Tasks); !reflect.DeepEqual(got, []string{"TSK-002", "TSK-001", "TSK-003"}) {
		t.Fatalf("task order = %v; want [TSK-002 TSK-001 TSK-003]", got)
	}
	if result.Tasks[0].Entity.Description != "Preserve complete entities." || result.Tasks[0].Contract.Order != 1 {
		t.Errorf("task lost entity or contract data: %+v", result.Tasks[0])
	}
	if !reflect.DeepEqual(result.Tasks[2].PrerequisiteIDs, []string{"TSK-001"}) {
		t.Errorf("TSK-003 prerequisites = %v; want [TSK-001]", result.Tasks[2].PrerequisiteIDs)
	}
	if got := entityIDs(result.Scope); !reflect.DeepEqual(got, []string{"REQ-001", "REQ-002"}) {
		t.Errorf("scope = %v; want [REQ-001 REQ-002]", got)
	}
	if result.Scope[0].Description == "" || len(result.Scope[0].Metadata) == 0 || result.Scope[1].Description == "" || len(result.Scope[1].Metadata) == 0 {
		t.Errorf("scope dropped architecture data: %+v", result.Scope)
	}
	if !reflect.DeepEqual(result.Delivery, []string{"REQ-002"}) {
		t.Errorf("delivery = %v; want [REQ-002]", result.Delivery)
	}
	if !reflect.DeepEqual(result.ReadyTaskIDs, []string{"TSK-002", "TSK-001"}) {
		t.Errorf("ready = %v; want [TSK-002 TSK-001]", result.ReadyTaskIDs)
	}
	if !reflect.DeepEqual(result.BlockedTaskIDs, []string{"TSK-003"}) || !reflect.DeepEqual(result.Blockers, map[string][]string{"TSK-003": {"TSK-001"}}) {
		t.Errorf("blocked = %v blockers = %v; want TSK-003 blocked by TSK-001", result.BlockedTaskIDs, result.Blockers)
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal PhaseContextResult: %v", err)
	}
	var roundTrip specgraph.PhaseContextResult
	if err := json.Unmarshal(payload, &roundTrip); err != nil {
		t.Fatalf("unmarshal PhaseContextResult: %v", err)
	}
	canonicalResult, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal canonical result: %v", err)
	}
	canonicalRoundTrip, err := json.Marshal(roundTrip)
	if err != nil {
		t.Fatalf("marshal canonical round trip: %v", err)
	}
	if !bytes.Equal(canonicalRoundTrip, canonicalResult) {
		t.Errorf("JSON round-trip lost data\ngot:  %s\nwant: %s", canonicalRoundTrip, canonicalResult)
	}
	t.Logf("phase context JSON:\n%s", payload)
}

func TestPhaseContextRejectsCycle(t *testing.T) {
	engine := openTestEngine(t)
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Plan"})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"})
	createPhaseContextTask(t, engine, "TSK-001", 1, "First cycle member.")
	createPhaseContextTask(t, engine, "TSK-002", 2, "Second cycle member.")
	for _, relation := range []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: "TSK-001", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-002", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-001", To: "TSK-002", Type: "task_depends_on"},
		{From: "TSK-002", To: "TSK-001", Type: "task_depends_on"},
	} {
		addPhaseContextRelation(t, engine, relation)
	}

	result, err := engine.PhaseContext("PHS-001")
	assertPhaseContextStateError(t, err, "task_graph", "TSK-001", "TSK-002")
	if !reflect.DeepEqual(result, specgraph.PhaseContextResult{}) {
		t.Errorf("cycle returned partial context: %+v", result)
	}
}

func TestPhaseContextRejectsAmbiguousParent(t *testing.T) {
	tests := []struct {
		name      string
		planIDs   []string
		wantParts []string
	}{
		{name: "zero plans", wantParts: []string{"PHS-001", "[]"}},
		{name: "multiple plans", planIDs: []string{"PLN-002", "PLN-001"}, wantParts: []string{"PHS-001", "PLN-001", "PLN-002"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := openTestEngine(t)
			createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"})
			for _, planID := range test.planIDs {
				createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "plan", ID: planID, Title: planID})
				addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "PHS-001", To: planID, Type: "belongs_to"})
			}

			_, err := engine.PhaseContext("PHS-001")
			assertPhaseContextStateError(t, err, test.wantParts...)
		})
	}
}

func TestPhaseContextRejectsMalformedContract(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Plan"})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"})
	createPhaseContextTask(t, engine, "TSK-001", 1, "Malformed after persistence.")
	addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"})
	addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "TSK-001", To: "PHS-001", Type: "belongs_to"})
	if err := engine.Close(); err != nil {
		t.Fatalf("close fixture engine: %v", err)
	}

	path := taskEntityPath(root, "TSK-001")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read task fixture: %v", err)
	}
	malformed := strings.Replace(string(data), "order = 1", "order = 0", 1)
	if malformed == string(data) {
		t.Fatal("task fixture did not contain metadata order")
	}
	if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
		t.Fatalf("write malformed task fixture: %v", err)
	}
	engine, err = specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("reopen malformed fixture: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	_, err = engine.PhaseContext("PHS-001")
	assertPhaseContextStateError(t, err, "task_contract", "TSK-001")
}

func TestPhaseContextTasklessUsesDirectEffectiveScope(t *testing.T) {
	engine := openTestEngine(t)
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Legacy plan"})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Legacy phase"})
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Legacy requirement"})
	addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"})
	addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "PHS-001", To: "REQ-001", Type: "covers"})
	addPhaseContextRelation(t, engine, specgraph.AddRelationRequest{From: "PHS-001", To: "REQ-001", Type: "delivers"})

	result, err := engine.PhaseContext("PHS-001")
	if err != nil {
		t.Fatalf("PhaseContext: %v", err)
	}
	if !reflect.DeepEqual(result.Tasks, []specgraph.TaskContext{}) {
		t.Errorf("tasks = %#v; want empty slice", result.Tasks)
	}
	if !reflect.DeepEqual(entityIDs(result.Scope), []string{"REQ-001"}) || !reflect.DeepEqual(result.Delivery, []string{"REQ-001"}) {
		t.Errorf("scope = %v delivery = %v; want direct REQ-001 mapping", entityIDs(result.Scope), result.Delivery)
	}
	if !reflect.DeepEqual(result.ReadyTaskIDs, []string{}) || !reflect.DeepEqual(result.BlockedTaskIDs, []string{}) || len(result.Blockers) != 0 {
		t.Errorf("taskless scheduling data is not empty: ready=%v blocked=%v blockers=%v", result.ReadyTaskIDs, result.BlockedTaskIDs, result.Blockers)
	}
}

func createPhaseContextEntity(t *testing.T, engine *specgraph.Engine, request specgraph.CreateEntityRequest) {
	t.Helper()
	if _, err := engine.CreateEntity(context.Background(), request); err != nil {
		t.Fatalf("create %s: %v", request.ID, err)
	}
}

func createPhaseContextTask(t *testing.T, engine *specgraph.Engine, id string, order int, description string) {
	t.Helper()
	contract := validTaskContract()
	contract.Order = order
	contract.Instructions = []string{description}
	createPhaseContextEntity(t, engine, specgraph.CreateEntityRequest{
		Type: "task", ID: id, Title: id, Description: description, Metadata: marshalTaskContract(t, contract),
	})
}

func addPhaseContextRelation(t *testing.T, engine *specgraph.Engine, request specgraph.AddRelationRequest) {
	t.Helper()
	if _, err := engine.AddRelation(context.Background(), request); err != nil {
		t.Fatalf("add %s %s->%s: %v", request.Type, request.From, request.To, err)
	}
}

func taskContextIDs(tasks []specgraph.TaskContext) []string {
	ids := make([]string, len(tasks))
	for index, task := range tasks {
		ids[index] = task.Entity.ID
	}
	return ids
}

func entityIDs(entities []model.Entity) []string {
	ids := make([]string, len(entities))
	for index, entity := range entities {
		ids[index] = entity.ID
	}
	return ids
}

func assertPhaseContextStateError(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected PhaseContext error")
	}
	var specgraphError *specgraph.Error
	if !errors.As(err, &specgraphError) {
		t.Fatalf("error type = %T; want *specgraph.Error", err)
	}
	if specgraphError.Code != specgraph.CodeInvalidState {
		t.Errorf("error code = %q; want %q", specgraphError.Code, specgraph.CodeInvalidState)
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not name %q", err, part)
		}
	}
}

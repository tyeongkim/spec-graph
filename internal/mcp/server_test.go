package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func newTestEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	engine, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

// callTool drives the handler the server actually registered, so a test covers
// argument binding and result encoding rather than the handler body alone.
func callTool(t *testing.T, engine *specgraph.Engine, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	tool := NewSpecGraphServer(engine, "test").GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q is not registered", name)
	}

	var request mcp.CallToolRequest
	request.Params.Name = name
	request.Params.Arguments = args

	result, err := tool.Handler(context.Background(), request)
	if err != nil {
		t.Fatalf("%s returned a transport error: %v", name, err)
	}
	return result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result carries no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, want mcp.TextContent", result.Content[0])
	}
	return text.Text
}

func callToolExpectingSuccess(t *testing.T, engine *specgraph.Engine, name string, args map[string]any) string {
	t.Helper()
	result := callTool(t, engine, name, args)
	if result.IsError {
		t.Fatalf("%s failed: %s", name, resultText(t, result))
	}
	return resultText(t, result)
}

func callToolExpectingError(t *testing.T, engine *specgraph.Engine, name string, args map[string]any) string {
	t.Helper()
	result := callTool(t, engine, name, args)
	if !result.IsError {
		t.Fatalf("%s succeeded, want an error: %s", name, resultText(t, result))
	}
	return resultText(t, result)
}

func taskContract(order int, instruction string) json.RawMessage {
	contract, err := json.Marshal(map[string]any{
		"order":        order,
		"instructions": []string{instruction},
		"acceptance":   []string{"The scoped behavior passes verification."},
		"must_not":     []string{},
		"references":   []string{},
		"qa":           []map[string]string{{"command": "go test ./...", "expected": "exit 0", "evidence": ""}},
	})
	if err != nil {
		panic(err)
	}
	return contract
}

// seedPhase builds an active plan with one phase, one task, and one covered
// requirement, which is the smallest graph the phase workflows accept.
func seedPhase(t *testing.T, engine *specgraph.Engine) {
	t.Helper()
	_, err := engine.ApplyBatch(context.Background(), specgraph.BatchRequest{
		Entities: []specgraph.BatchEntity{
			{CreateEntityRequest: specgraph.CreateEntityRequest{
				Type: "plan", ID: "PLN-001", Title: "Delivery plan", Status: "active",
			}},
			{CreateEntityRequest: specgraph.CreateEntityRequest{
				Type: "phase", ID: "PHS-001", Title: "First phase",
				Description: "Deliver the first slice.",
				Metadata:    json.RawMessage(`{"goal":"Ship the first slice","order":1,"exit_criteria":["All tests pass"]}`),
			}},
			{CreateEntityRequest: specgraph.CreateEntityRequest{
				Type: "requirement", ID: "REQ-001", Title: "Authenticate a user",
				Metadata: json.RawMessage(`{"priority":"must","kind":"functional"}`),
			}},
			{CreateEntityRequest: specgraph.CreateEntityRequest{
				Type: "task", ID: "TSK-001", Title: "Implement authentication",
				Description: "Implement the authentication path.",
				Metadata:    taskContract(1, "Implement the authentication path."),
			}},
		},
		Relations: []specgraph.BatchRelation{
			{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
			{From: "TSK-001", To: "PHS-001", Type: "belongs_to"},
			{From: "TSK-001", To: "REQ-001", Type: "covers"},
		},
	})
	if err != nil {
		t.Fatalf("seed phase: %v", err)
	}
}

func TestRegisteredToolNames(t *testing.T) {
	t.Parallel()

	registered := NewSpecGraphServer(newTestEngine(t), "test").ListTools()

	want := []string{
		"apply_batch", "change_impact", "delete_entity", "delete_relation",
		"get_entity", "list_entities", "list_relations", "next_phase",
		"phase_brief", "phase_gate", "plan_status", "query_path", "update_entity",
	}
	for _, name := range want {
		if _, ok := registered[name]; !ok {
			t.Errorf("tool %q is not registered", name)
		}
	}
	if len(registered) != len(want) {
		t.Errorf("registered %d tools, want %d: %v", len(registered), len(want), registered)
	}
}

// TestToolAnnotationsMatchToolEffects guards the advertised hints because mcp-go
// defaults destructiveHint to true, so an additive writer is mislabelled unless
// the tool sets it.
func TestToolAnnotationsMatchToolEffects(t *testing.T) {
	t.Parallel()

	registered := NewSpecGraphServer(newTestEngine(t), "test").ListTools()

	readOnly := map[string]bool{
		"plan_status": true, "phase_brief": true, "phase_gate": true,
		"change_impact": true, "get_entity": true, "list_entities": true,
		"list_relations": true, "query_path": true,
	}
	destructive := map[string]bool{"delete_entity": true, "delete_relation": true}

	for name, tool := range registered {
		annotations := tool.Tool.Annotations
		if annotations.ReadOnlyHint == nil || annotations.DestructiveHint == nil {
			t.Errorf("%s leaves a hint unset, so the client sees the library default", name)
			continue
		}
		if got := *annotations.ReadOnlyHint; got != readOnly[name] {
			t.Errorf("%s readOnlyHint is %v, want %v", name, got, readOnly[name])
		}
		if got := *annotations.DestructiveHint; got != destructive[name] {
			t.Errorf("%s destructiveHint is %v, want %v", name, got, destructive[name])
		}
	}
}

func TestApplyBatchResolvesRefsAcrossItems(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)

	text := callToolExpectingSuccess(t, engine, "apply_batch", map[string]any{
		"entities": []any{
			map[string]any{"ref": "auth", "type": "requirement", "title": "Authenticate a user"},
			map[string]any{"ref": "token", "type": "decision", "title": "Adopt JWT"},
		},
		"relations": []any{
			map[string]any{"from": "auth", "to": "token", "type": "constrained_by"},
		},
	})

	var result batchResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode batch result: %v", err)
	}
	if len(result.Entities) != 2 || len(result.Relations) != 1 {
		t.Fatalf("created %d entities and %d relations, want 2 and 1", len(result.Entities), len(result.Relations))
	}

	requirementID := result.Entities[0].Entity.ID
	decisionID := result.Entities[1].Entity.ID
	if result.Relations[0].FromID != requirementID || result.Relations[0].ToID != decisionID {
		t.Errorf("relation links %s->%s, want the generated %s->%s",
			result.Relations[0].FromID, result.Relations[0].ToID, requirementID, decisionID)
	}
	if result.Entities[0].Ref != "auth" {
		t.Errorf("ref is %q, want the caller's own \"auth\" so a generated ID maps back", result.Entities[0].Ref)
	}
}

// TestApplyBatchEmitsSnakeCaseKeys guards the wire format: specgraph.BatchResult
// declares no JSON tags, so returning it unconverted emits Go field names.
func TestApplyBatchEmitsSnakeCaseKeys(t *testing.T) {
	t.Parallel()

	text := callToolExpectingSuccess(t, newTestEngine(t), "apply_batch", map[string]any{
		"entities": []any{
			map[string]any{"ref": "auth", "type": "requirement", "title": "Authenticate a user"},
		},
	})

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("decode batch result: %v", err)
	}
	for _, key := range []string{"entities", "relations"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("result has no %q key: %s", key, text)
		}
	}
	for _, key := range []string{"Entities", "Relations"} {
		if _, ok := envelope[key]; ok {
			t.Errorf("result exposes the Go field name %q: %s", key, text)
		}
	}
}

func TestApplyBatchLeavesGraphUntouchedWhenAnItemFails(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)

	text := callToolExpectingError(t, engine, "apply_batch", map[string]any{
		"entities": []any{
			map[string]any{"type": "requirement", "id": "REQ-001", "title": "Authenticate a user"},
			map[string]any{"type": "requirement", "id": "REQ-001", "title": "Duplicate ID"},
		},
	})
	if !strings.Contains(text, "entity 1") {
		t.Errorf("error %q does not name the failing position", text)
	}

	if _, err := engine.GetEntity(context.Background(), "REQ-001"); err == nil {
		t.Error("REQ-001 was persisted; a failed batch must write nothing")
	}
}

func TestApplyBatchRejectsRefShapedLikeAnEntityID(t *testing.T) {
	t.Parallel()

	text := callToolExpectingError(t, newTestEngine(t), "apply_batch", map[string]any{
		"entities": []any{
			map[string]any{"ref": "REQ-001", "type": "requirement", "title": "Authenticate a user"},
		},
	})
	if !strings.Contains(text, "must not look like an entity ID") {
		t.Errorf("error %q does not explain the ref namespace rule", text)
	}
}

func TestPlanStatusReportsActivePlanAndPhase(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)
	if _, err := engine.PhaseNext(context.Background(), specgraph.PhaseNextRequest{Activate: true}); err != nil {
		t.Fatalf("activate phase: %v", err)
	}

	text := callToolExpectingSuccess(t, engine, "plan_status", map[string]any{})

	var result planStatusResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode plan status: %v", err)
	}
	if len(result.ActivePlans) != 1 || result.ActivePlans[0].ID != "PLN-001" {
		t.Errorf("active plans %+v, want only PLN-001", result.ActivePlans)
	}
	if len(result.ActivePhases) != 1 || result.ActivePhases[0].ID != "PHS-001" {
		t.Errorf("active phases %+v, want only PHS-001", result.ActivePhases)
	}
}

func TestPhaseBriefCarriesContextAndPhaseScopedIssues(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "phase_brief", map[string]any{"phase_id": "PHS-001"})

	var brief phaseBriefResult
	if err := json.Unmarshal([]byte(text), &brief); err != nil {
		t.Fatalf("decode phase brief: %v", err)
	}

	want, err := engine.PhaseContext(context.Background(), "PHS-001")
	if err != nil {
		t.Fatalf("engine phase context: %v", err)
	}
	if brief.Context.Phase.ID != want.Phase.ID || len(brief.Context.Tasks) != len(want.Tasks) {
		t.Errorf("brief context %+v does not match the engine result %+v", brief.Context, want)
	}
	if brief.Clean != (len(brief.Issues) == 0) {
		t.Errorf("clean is %v with %d issues", brief.Clean, len(brief.Issues))
	}
}

func TestPhaseToolsRejectANonPhaseID(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	for _, name := range []string{"phase_brief", "phase_gate"} {
		t.Run(name, func(t *testing.T) {
			text := callToolExpectingError(t, engine, name, map[string]any{"phase_id": "REQ-001"})
			if !strings.Contains(text, "not phase") {
				t.Errorf("error %q does not say the entity is not a phase", text)
			}
		})
	}
}

func TestPhaseGateWithholdsPassWhileDeliveryIsIncomplete(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "phase_gate", map[string]any{"phase_id": "PHS-001"})

	var gate phaseGateResult
	if err := json.Unmarshal([]byte(text), &gate); err != nil {
		t.Fatalf("decode phase gate: %v", err)
	}
	if gate.Passed {
		t.Error("gate passed while REQ-001 has no delivers relation")
	}
	if len(gate.Issues) == 0 {
		t.Error("gate reported no issues, so nothing tells the caller what to fix")
	}
}

func TestPhaseGateLeavesPhaseStatusUnchanged(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	callToolExpectingSuccess(t, engine, "phase_gate", map[string]any{"phase_id": "PHS-001"})

	phase, err := engine.GetEntity(context.Background(), "PHS-001")
	if err != nil {
		t.Fatalf("get phase: %v", err)
	}
	if string(phase.Status) != "draft" {
		t.Errorf("phase status is %q, want draft: the gate must not transition a phase", phase.Status)
	}
}

func TestChangeImpactReturnsNeighborsPerSource(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "change_impact", map[string]any{
		"sources": []any{"REQ-001"},
	})

	var result changeImpactResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode change impact: %v", err)
	}
	if result.Impact == nil {
		t.Fatal("impact is absent")
	}
	if _, ok := result.Neighbors["REQ-001"]; !ok {
		t.Errorf("neighbors %v lack an entry for the requested source", result.Neighbors)
	}
}

func TestChangeImpactRejectsAnInvalidDimension(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingError(t, engine, "change_impact", map[string]any{
		"sources":   []any{"REQ-001"},
		"dimension": "bogus",
	})
	if !strings.Contains(text, "structural") {
		t.Errorf("error %q does not list the accepted dimensions", text)
	}
}

func TestUpdateEntityReportsABlockedTransitionWithoutWriting(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)
	if _, err := engine.PhaseNext(context.Background(), specgraph.PhaseNextRequest{Activate: true}); err != nil {
		t.Fatalf("activate phase: %v", err)
	}

	text := callToolExpectingSuccess(t, engine, "update_entity", map[string]any{
		"id":     "PHS-001",
		"status": "resolved",
	})

	var result updateEntityResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode update result: %v", err)
	}
	if !result.Blocked {
		t.Fatalf("outcome %q, want a blocked resolution while REQ-001 is undelivered", result.Outcome)
	}

	phase, err := engine.GetEntity(context.Background(), "PHS-001")
	if err != nil {
		t.Fatalf("get phase: %v", err)
	}
	if string(phase.Status) != "active" {
		t.Errorf("phase status is %q, want active to remain after a blocked write", phase.Status)
	}
}

// TestUpdateEntityOmitsGateReportWhenNoGateRan pins the wire format: a typed nil
// in an `any` field would serialize as "gate_report":null on every success.
func TestUpdateEntityOmitsGateReportWhenNoGateRan(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "update_entity", map[string]any{
		"id":    "REQ-001",
		"title": "Authenticate a user with a token",
	})
	if strings.Contains(text, "gate_report") {
		t.Errorf("no gate ran, so gate_report should be absent: %s", text)
	}
}

func TestUpdateEntityDeprecatesThroughStatus(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	callToolExpectingSuccess(t, engine, "update_entity", map[string]any{
		"id":     "REQ-001",
		"status": "deprecated",
	})

	entity, err := engine.GetEntity(context.Background(), "REQ-001")
	if err != nil {
		t.Fatalf("get requirement: %v", err)
	}
	if string(entity.Status) != "deprecated" {
		t.Errorf("status is %q, want deprecated", entity.Status)
	}
}

func TestDeleteEntityIsRefusedWhileRelationsReferenceIt(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	callToolExpectingError(t, engine, "delete_entity", map[string]any{"id": "REQ-001"})

	if _, err := engine.GetEntity(context.Background(), "REQ-001"); err != nil {
		t.Errorf("REQ-001 was removed despite the refusal: %v", err)
	}
}

func TestNextPhaseActivatesTheSelectedPhase(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "next_phase", map[string]any{"activate": true})

	var result specgraph.PhaseNextResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode next phase: %v", err)
	}
	if result.Phase.ID != "PHS-001" || !result.Activated {
		t.Fatalf("selected %q with activated=%v, want PHS-001 activated", result.Phase.ID, result.Activated)
	}

	phase, err := engine.GetEntity(context.Background(), "PHS-001")
	if err != nil {
		t.Fatalf("get phase: %v", err)
	}
	if string(phase.Status) != "active" {
		t.Errorf("phase status is %q, want active", phase.Status)
	}
}

func TestQueryPathFindsTheRelationChain(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "query_path", map[string]any{
		"from_id": "TSK-001",
		"to_id":   "REQ-001",
	})
	if !strings.Contains(text, "REQ-001") {
		t.Errorf("path result %q does not reach the target", text)
	}
}

func TestListEntitiesFiltersByType(t *testing.T) {
	t.Parallel()
	engine := newTestEngine(t)
	seedPhase(t, engine)

	text := callToolExpectingSuccess(t, engine, "list_entities", map[string]any{"type": "requirement"})

	var result jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("decode entity list: %v", err)
	}
	if result.Count != 1 || result.Entities[0].ID != "REQ-001" {
		t.Errorf("listed %+v, want only REQ-001", result.Entities)
	}
}

func TestListEntitiesRejectsAnInvalidLayer(t *testing.T) {
	t.Parallel()

	text := callToolExpectingError(t, newTestEngine(t), "list_entities", map[string]any{"layer": "bogus"})
	if !strings.Contains(text, "layer") {
		t.Errorf("error %q does not mention the rejected layer", text)
	}
}

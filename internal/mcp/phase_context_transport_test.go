package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestPhaseContextTransportParity(t *testing.T) {
	engine := newTestEngine(t)
	seedPhaseContextMCP(t, engine)
	want, err := engine.PhaseContext("PHS-001")
	if err != nil {
		t.Fatalf("direct PhaseContext: %v", err)
	}

	result, err := handlePhaseContext(engine)(context.Background(), callRequest(map[string]any{"id": "PHS-001"}))
	if err != nil {
		t.Fatalf("phase_context handler: %v", err)
	}
	if result.IsError {
		t.Fatalf("phase_context tool error: %s", resultText(t, result))
	}
	var got specgraph.PhaseContextResult
	if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
		t.Fatalf("unmarshal phase_context result: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MCP phase context differs from engine result\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestPhaseContextTransportErrors(t *testing.T) {
	engine := newTestEngine(t)
	if _, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Requirement"}); err != nil {
		t.Fatalf("create requirement: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantMessage string
	}{
		{name: "non-phase ID", id: "REQ-001", wantMessage: "expected \"PHS\""},
		{name: "missing ID", id: "PHS-999", wantMessage: "not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := handlePhaseContext(engine)(context.Background(), callRequest(map[string]any{"id": test.id}))
			if err != nil {
				t.Fatalf("phase_context handler: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected MCP tool error, got: %s", resultText(t, result))
			}
			text := resultText(t, result)
			if !strings.Contains(text, test.wantMessage) {
				t.Errorf("error %q does not contain %q", text, test.wantMessage)
			}
			if strings.Contains(text, `"plan"`) {
				t.Errorf("unexpected result payload in MCP error: %s", text)
			}
		})
	}
}

func seedPhaseContextMCP(t *testing.T, engine *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()
	entities := []specgraph.CreateEntityRequest{
		{Type: "plan", ID: "PLN-001", Title: "Delivery plan", Description: "Complete parent plan.", Metadata: json.RawMessage(`{"owner":"platform"}`)},
		{Type: "phase", ID: "PHS-001", Title: "Context phase", Description: "Complete child phase.", Metadata: json.RawMessage(`{"goal":"Expose phase context","order":1,"exit_criteria":["Context is deterministic"]}`)},
		{Type: "requirement", ID: "REQ-001", Title: "First requirement", Description: "First covered entity.", Metadata: json.RawMessage(`{"priority":"must","kind":"functional","owner":"platform"}`)},
		{Type: "requirement", ID: "REQ-002", Title: "Second requirement", Description: "Second covered entity.", Metadata: json.RawMessage(`{"priority":"should","kind":"non_functional","owner":"quality"}`)},
		{Type: "task", ID: "TSK-001", Title: "Implement result", Description: "Implement the engine result.", Metadata: mcpTaskContract(2, "Implement the engine result.")},
		{Type: "task", ID: "TSK-002", Title: "Preserve entities", Description: "Preserve complete entities.", Metadata: mcpTaskContract(1, "Preserve complete entities.")},
		{Type: "task", ID: "TSK-003", Title: "Consume result", Description: "Consume the engine result.", Metadata: mcpTaskContract(1, "Consume the engine result.")},
	}
	for _, entity := range entities {
		if _, err := engine.CreateEntity(ctx, entity); err != nil {
			t.Fatalf("create %s: %v", entity.ID, err)
		}
	}
	relations := []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: "TSK-001", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-002", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-003", To: "PHS-001", Type: "belongs_to"},
		{From: "TSK-003", To: "TSK-001", Type: "task_depends_on"},
		{From: "TSK-001", To: "REQ-001", Type: "covers"},
		{From: "TSK-002", To: "REQ-002", Type: "covers"},
		{From: "TSK-002", To: "REQ-002", Type: "delivers"},
		{From: "TSK-003", To: "REQ-001", Type: "covers"},
	}
	for _, relation := range relations {
		if _, err := engine.AddRelation(ctx, relation); err != nil {
			t.Fatalf("add %s %s->%s: %v", relation.Type, relation.From, relation.To, err)
		}
	}
}

func mcpTaskContract(order int, instruction string) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"order": order, "instructions": []string{instruction}, "acceptance": []string{"Transport matches engine result."},
		"must_not": []string{}, "references": []string{},
		"qa": []map[string]string{{"command": "go test ./...", "expected": "exit 0", "evidence": ""}},
	})
	return payload
}

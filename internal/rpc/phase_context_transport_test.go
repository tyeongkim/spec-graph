package rpc

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestPhaseContextTransportParity(t *testing.T) {
	engine := newTestEngine(t)
	seedPhaseContextRPC(t, engine)
	want, err := engine.PhaseContext("PHS-001")
	if err != nil {
		t.Fatalf("direct PhaseContext: %v", err)
	}

	dispatcher := NewDispatcher(engine)
	payload, notification := dispatcher.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"phase.context","params":{"id":"PHS-001"}}`))
	if notification {
		t.Fatal("phase.context should not be a notification")
	}
	var envelope response
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal RPC response: %v", err)
	}
	if envelope.Error != nil {
		t.Fatalf("phase.context returned error: %+v", envelope.Error)
	}
	var got specgraph.PhaseContextResult
	if err := json.Unmarshal(envelope.Result, &got); err != nil {
		t.Fatalf("unmarshal phase.context result: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RPC phase context differs from engine result\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestPhaseContextTransportErrors(t *testing.T) {
	engine := newTestEngine(t)
	if _, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Requirement"}); err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	dispatcher := NewDispatcher(engine)

	tests := []struct {
		name     string
		id       string
		wantCode string
	}{
		{name: "non-phase ID", id: "REQ-001", wantCode: string(specgraph.CodeInvalidInput)},
		{name: "missing ID", id: "PHS-999", wantCode: string(specgraph.CodeNotFound)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "phase.context", "params": map[string]string{"id": test.id}}
			requestJSON, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			payload, _ := dispatcher.Handle(context.Background(), requestJSON)
			var envelope response
			if err := json.Unmarshal(payload, &envelope); err != nil {
				t.Fatalf("unmarshal RPC response: %v", err)
			}
			if len(envelope.Result) != 0 {
				t.Errorf("unexpected result payload: %s", envelope.Result)
			}
			if envelope.Error == nil {
				t.Fatal("expected RPC error")
			}
			if envelope.Error.Code != codeInvalidParams {
				t.Errorf("RPC error code = %d; want %d", envelope.Error.Code, codeInvalidParams)
			}
			dataJSON, err := json.Marshal(envelope.Error.Data)
			if err != nil {
				t.Fatalf("marshal error data: %v", err)
			}
			var data errorData
			if err := json.Unmarshal(dataJSON, &data); err != nil {
				t.Fatalf("unmarshal error data: %v", err)
			}
			if data.Code != test.wantCode {
				t.Errorf("engine error code = %q; want %q", data.Code, test.wantCode)
			}
		})
	}
}

func seedPhaseContextRPC(t *testing.T, engine *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()
	entities := []specgraph.CreateEntityRequest{
		{Type: "plan", ID: "PLN-001", Title: "Delivery plan", Description: "Complete parent plan.", Metadata: json.RawMessage(`{"owner":"platform"}`)},
		{Type: "phase", ID: "PHS-001", Title: "Context phase", Description: "Complete child phase.", Metadata: json.RawMessage(`{"goal":"Expose phase context","order":1,"exit_criteria":["Context is deterministic"]}`)},
		{Type: "requirement", ID: "REQ-001", Title: "First requirement", Description: "First covered entity.", Metadata: json.RawMessage(`{"priority":"must","kind":"functional","owner":"platform"}`)},
		{Type: "requirement", ID: "REQ-002", Title: "Second requirement", Description: "Second covered entity.", Metadata: json.RawMessage(`{"priority":"should","kind":"non_functional","owner":"quality"}`)},
		{Type: "task", ID: "TSK-001", Title: "Implement result", Description: "Implement the engine result.", Metadata: rpcTaskContract(2, "Implement the engine result.")},
		{Type: "task", ID: "TSK-002", Title: "Preserve entities", Description: "Preserve complete entities.", Metadata: rpcTaskContract(1, "Preserve complete entities.")},
		{Type: "task", ID: "TSK-003", Title: "Consume result", Description: "Consume the engine result.", Metadata: rpcTaskContract(1, "Consume the engine result.")},
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

func rpcTaskContract(order int, instruction string) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"order": order, "instructions": []string{instruction}, "acceptance": []string{"Transport matches engine result."},
		"must_not": []string{}, "references": []string{},
		"qa": []map[string]string{{"command": "go test ./...", "expected": "exit 0", "evidence": ""}},
	})
	return payload
}

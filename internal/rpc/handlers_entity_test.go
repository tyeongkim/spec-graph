package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type updateRPCResult struct {
	Entity struct {
		Status model.EntityStatus `json:"status"`
	} `json:"entity"`
	Outcome    string          `json:"outcome"`
	Blocked    bool            `json:"blocked"`
	GateReport json.RawMessage `json:"gate_report"`
}

func callEntityUpdate(t *testing.T, dispatcher *Dispatcher, params string) updateRPCResult {
	t.Helper()
	request := `{"jsonrpc":"2.0","id":1,"method":"entity.update","params":` + params + `}`
	encoded, notification := dispatcher.Handle(context.Background(), []byte(request))
	if notification {
		t.Fatal("entity.update returned a notification")
	}
	var envelope response
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Error != nil {
		t.Fatalf("entity.update error: %+v", envelope.Error)
	}
	var result updateRPCResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("unmarshal update result: %v", err)
	}
	return result
}

func TestEntityUpdateRPCDistinguishesForcedApplyFromBlocked(t *testing.T) {
	t.Run("completion force is applied", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "phase", ID: "PHS-001", Title: "Phase", Status: "active",
		}); err != nil {
			t.Fatalf("create phase: %v", err)
		}
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "question", ID: "QST-001", Title: "Question", Status: "active",
		}); err != nil {
			t.Fatalf("create question: %v", err)
		}
		if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "PHS-001", To: "QST-001", Type: "covers",
		}); err != nil {
			t.Fatalf("add covers: %v", err)
		}

		result := callEntityUpdate(t, NewDispatcher(engine),
			`{"id":"PHS-001","status":"resolved","force":true,"reason":"Accept question"}`)
		if result.Outcome != "applied_with_force" || result.Blocked {
			t.Fatalf("outcome = %q, blocked = %v", result.Outcome, result.Blocked)
		}
		if result.Entity.Status != model.EntityStatusResolved {
			t.Fatalf("status = %q; want resolved", result.Entity.Status)
		}
		if len(result.GateReport) == 0 {
			t.Fatal("forced update omitted gate report")
		}
	})

	t.Run("structural force is blocked", func(t *testing.T) {
		engine := newTestEngine(t)
		if _, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
			Type: "phase", ID: "PHS-001", Title: "Resolved phase", Status: "resolved",
		}); err != nil {
			t.Fatalf("create phase: %v", err)
		}

		result := callEntityUpdate(t, NewDispatcher(engine),
			`{"id":"PHS-001","status":"active","force":true,"reason":"Attempt reopen"}`)
		if result.Outcome != "blocked" || !result.Blocked {
			t.Fatalf("outcome = %q, blocked = %v", result.Outcome, result.Blocked)
		}
		if result.Entity.Status != model.EntityStatusResolved {
			t.Fatalf("status = %q; want resolved", result.Entity.Status)
		}
	})
}

package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type batchRPCResult struct {
	Entities []struct {
		Ref    string `json:"ref"`
		Entity struct {
			ID     string             `json:"id"`
			Type   model.EntityType   `json:"type"`
			Status model.EntityStatus `json:"status"`
		} `json:"entity"`
	} `json:"entities"`
	Relations []struct {
		FromID string `json:"from_id"`
		ToID   string `json:"to_id"`
		Type   string `json:"type"`
	} `json:"relations"`
}

func callBatchApply(t *testing.T, dispatcher *Dispatcher, params string) (batchRPCResult, *rpcError) {
	t.Helper()
	request := `{"jsonrpc":"2.0","id":1,"method":"batch.apply","params":` + params + `}`
	encoded, notification := dispatcher.Handle(context.Background(), []byte(request))
	if notification {
		t.Fatal("batch.apply returned a notification")
	}

	var envelope response
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Error != nil {
		return batchRPCResult{}, envelope.Error
	}

	var result batchRPCResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("unmarshal batch result: %v", err)
	}
	return result, nil
}

func TestBatchApplyRPCWiresRefsAcrossOneRequest(t *testing.T) {
	engine := newTestEngine(t)

	result, rpcErr := callBatchApply(t, NewDispatcher(engine), `{
		"entities": [
			{"ref": "plan", "type": "plan", "title": "v1 Delivery", "status": "active"},
			{"ref": "phase", "type": "phase", "title": "Phase 1"}
		],
		"relations": [
			{"from": "phase", "to": "plan", "type": "belongs_to"}
		]
	}`)
	if rpcErr != nil {
		t.Fatalf("batch.apply error: %+v", rpcErr)
	}

	ids := make(map[string]string, len(result.Entities))
	for _, item := range result.Entities {
		ids[item.Ref] = item.Entity.ID
	}
	if ids["plan"] == "" || ids["phase"] == "" {
		t.Fatalf("entities = %+v, want an ID reported for each ref", result.Entities)
	}

	if len(result.Relations) != 1 {
		t.Fatalf("relations = %+v, want one relation", result.Relations)
	}
	if got := result.Relations[0]; got.FromID != ids["phase"] || got.ToID != ids["plan"] {
		t.Errorf("relation = %s -> %s, want %s -> %s", got.FromID, got.ToID, ids["phase"], ids["plan"])
	}

	for _, item := range result.Entities {
		if item.Ref == "plan" && item.Entity.Status != model.EntityStatusActive {
			t.Errorf("plan status = %q, want %q", item.Entity.Status, model.EntityStatusActive)
		}
	}
}

func TestBatchApplyRPCReportsFailureAsInvalidParams(t *testing.T) {
	engine := newTestEngine(t)

	_, rpcErr := callBatchApply(t, NewDispatcher(engine), `{
		"entities": [
			{"ref": "req", "type": "requirement", "title": "Requirement"},
			{"ref": "plan", "type": "plan", "title": "Plan"}
		],
		"relations": [
			{"from": "req", "to": "plan", "type": "belongs_to"}
		]
	}`)
	if rpcErr == nil {
		t.Fatal("batch.apply error = nil, want the invalid relation rejected")
	}
	if rpcErr.Code != codeInvalidParams {
		t.Errorf("error code = %d, want %d", rpcErr.Code, codeInvalidParams)
	}
	if !strings.Contains(rpcErr.Message, "relation 0") {
		t.Errorf("error message = %q, want it to locate the failing relation", rpcErr.Message)
	}

	entities, count, err := engine.ListEntities(context.Background(), specgraph.ListEntitiesRequest{})
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if count != 0 {
		t.Errorf("entity count = %d, want 0; the failed batch left state behind: %+v", count, entities)
	}
}

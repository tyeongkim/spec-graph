package jsoncontract

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestEntityResponseJSON(t *testing.T) {
	resp := EntityResponse{
		Entity: model.Entity{
			ID:     "REQ-001",
			Type:   model.EntityTypeRequirement,
			Title:  "Test",
			Status: model.EntityStatusDraft,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["entity"]; !ok {
		t.Error("expected top-level 'entity' key")
	}
}

func TestEntityListResponseJSON(t *testing.T) {
	resp := EntityListResponse{
		Entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "A", Status: model.EntityStatusDraft},
		},
		Count: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["entities"]; !ok {
		t.Error("expected top-level 'entities' key")
	}
	if _, ok := raw["count"]; !ok {
		t.Error("expected top-level 'count' key")
	}
}

func TestRelationResponseJSON(t *testing.T) {
	resp := RelationResponse{
		Relation: model.Relation{
			ID:     1,
			FromID: "REQ-001",
			ToID:   "DEC-001",
			Type:   model.RelationDependsOn,
			Weight: 1.0,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["relation"]; !ok {
		t.Error("expected top-level 'relation' key")
	}
}

func TestRelationListResponseJSON(t *testing.T) {
	resp := RelationListResponse{
		Relations: []model.Relation{
			{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn, Weight: 1.0},
		},
		Count: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["relations"]; !ok {
		t.Error("expected top-level 'relations' key")
	}
	if _, ok := raw["count"]; !ok {
		t.Error("expected top-level 'count' key")
	}
}

func TestErrorResponseJSON(t *testing.T) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    "entity_not_found",
			Message: "entity REQ-001 not found",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["error"]; !ok {
		t.Error("expected top-level 'error' key")
	}

	var errObj map[string]json.RawMessage
	if err := json.Unmarshal(raw["error"], &errObj); err != nil {
		t.Fatalf("Unmarshal error object failed: %v", err)
	}
	if _, ok := errObj["code"]; !ok {
		t.Error("expected 'code' in error object")
	}
	if _, ok := errObj["message"]; !ok {
		t.Error("expected 'message' in error object")
	}
}

func TestDeleteResponseJSON(t *testing.T) {
	resp := DeleteResponse{Deleted: "REQ-001"}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["deleted"]; !ok {
		t.Error("expected top-level 'deleted' key")
	}
}

func TestInitResponseJSON(t *testing.T) {
	resp := InitResponse{Initialized: true, Path: "/tmp/graph.db"}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := raw["initialized"]; !ok {
		t.Error("expected top-level 'initialized' key")
	}
	if _, ok := raw["path"]; !ok {
		t.Error("expected top-level 'path' key")
	}
}

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

func TestImpactResponseJSON(t *testing.T) {
	resp := ImpactResponse{
		Sources: []string{"REQ-001"},
		Affected: []ImpactAffected{
			{
				ID:            "DEC-001",
				Type:          "decision",
				Depth:         1,
				Path:          []string{"REQ-001", "DEC-001"},
				RelationChain: []string{"depends_on"},
				Impact: ImpactScores{
					Overall:    "high",
					Structural: "medium",
					Behavioral: "high",
					Planning:   "low",
				},
				Reason: "direct dependency",
			},
		},
		Summary: ImpactSummary{
			Total:    1,
			ByType:   map[string]int{"decision": 1},
			ByImpact: map[string]int{"high": 1},
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

	for _, key := range []string{"sources", "affected", "summary"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var affected []map[string]json.RawMessage
	if err := json.Unmarshal(raw["affected"], &affected); err != nil {
		t.Fatalf("Unmarshal affected failed: %v", err)
	}
	for _, key := range []string{"id", "type", "depth", "path", "relation_chain", "impact", "reason"} {
		if _, ok := affected[0][key]; !ok {
			t.Errorf("expected affected[0] %q key", key)
		}
	}

	var impact map[string]json.RawMessage
	if err := json.Unmarshal(affected[0]["impact"], &impact); err != nil {
		t.Fatalf("Unmarshal impact failed: %v", err)
	}
	for _, key := range []string{"overall", "structural", "behavioral", "planning"} {
		if _, ok := impact[key]; !ok {
			t.Errorf("expected impact %q key", key)
		}
	}

	var summary map[string]json.RawMessage
	if err := json.Unmarshal(raw["summary"], &summary); err != nil {
		t.Fatalf("Unmarshal summary failed: %v", err)
	}
	for _, key := range []string{"total", "by_type", "by_impact"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("expected summary %q key", key)
		}
	}
}

func TestValidateResponseJSON(t *testing.T) {
	resp := ValidateResponse{
		Valid: false,
		Issues: []ValidateIssue{
			{
				Check:    "orphan_entity",
				Severity: "warning",
				Entity:   "REQ-002",
				Message:  "entity has no relations",
			},
		},
		Summary: ValidateSummary{
			TotalIssues: 1,
			BySeverity:  map[string]int{"warning": 1},
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

	for _, key := range []string{"valid", "issues", "summary"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var issues []map[string]json.RawMessage
	if err := json.Unmarshal(raw["issues"], &issues); err != nil {
		t.Fatalf("Unmarshal issues failed: %v", err)
	}
	for _, key := range []string{"check", "severity", "entity", "message"} {
		if _, ok := issues[0][key]; !ok {
			t.Errorf("expected issues[0] %q key", key)
		}
	}

	var summary map[string]json.RawMessage
	if err := json.Unmarshal(raw["summary"], &summary); err != nil {
		t.Fatalf("Unmarshal summary failed: %v", err)
	}
	for _, key := range []string{"total_issues", "by_severity"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("expected summary %q key", key)
		}
	}
}

func TestImpactResponseEmptyAffected(t *testing.T) {
	resp := ImpactResponse{
		Sources:  []string{"REQ-001"},
		Affected: make([]ImpactAffected, 0),
		Summary:  ImpactSummary{Total: 0, ByType: map[string]int{}, ByImpact: map[string]int{}},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if string(raw["affected"]) != "[]" {
		t.Errorf("expected affected to serialize as [], got %s", raw["affected"])
	}
}

func TestValidateResponseNoIssues(t *testing.T) {
	resp := ValidateResponse{
		Valid:   true,
		Issues:  make([]ValidateIssue, 0),
		Summary: ValidateSummary{TotalIssues: 0, BySeverity: map[string]int{}},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if string(raw["valid"]) != "true" {
		t.Errorf("expected valid=true, got %s", raw["valid"])
	}
	if string(raw["issues"]) != "[]" {
		t.Errorf("expected issues to serialize as [], got %s", raw["issues"])
	}
}

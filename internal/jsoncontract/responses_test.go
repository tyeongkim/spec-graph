package jsoncontract

import (
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
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

func TestChangesetResponseJSON(t *testing.T) {
	raw1 := json.RawMessage(`{"id":"REQ-001"}`)
	resp := ChangesetResponse{
		Changeset: ChangesetDetail{
			ID:        "cs-001",
			Reason:    "initial",
			Actor:     "user1",
			Source:    "cli",
			CreatedAt: "2026-01-01T00:00:00Z",
		},
		EntityEntries: []EntityHistoryEntry{
			{
				ID:          1,
				ChangesetID: "cs-001",
				EntityID:    "REQ-001",
				Action:      "create",
				Before:      nil,
				After:       &raw1,
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
		},
		RelationEntries: []RelationHistoryEntry{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"changeset", "entity_entries", "relation_entries"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var cs map[string]json.RawMessage
	if err := json.Unmarshal(out["changeset"], &cs); err != nil {
		t.Fatalf("Unmarshal changeset failed: %v", err)
	}
	for _, key := range []string{"id", "reason", "actor", "source", "created_at"} {
		if _, ok := cs[key]; !ok {
			t.Errorf("expected changeset %q key", key)
		}
	}
}

func TestChangesetDetailOmitempty(t *testing.T) {
	resp := ChangesetDetail{
		ID:        "cs-002",
		Reason:    "fix",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := out["actor"]; ok {
		t.Error("expected 'actor' to be omitted when empty")
	}
	if _, ok := out["source"]; ok {
		t.Error("expected 'source' to be omitted when empty")
	}
}

func TestChangesetListResponseJSON(t *testing.T) {
	resp := ChangesetListResponse{
		Changesets: []ChangesetDetail{
			{ID: "cs-001", Reason: "init", CreatedAt: "2026-01-01T00:00:00Z"},
		},
		Count: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"changesets", "count"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}
}

func TestEntityHistoryResponseJSON(t *testing.T) {
	after := json.RawMessage(`{"id":"REQ-001","title":"New"}`)
	resp := EntityHistoryResponse{
		EntityID: "REQ-001",
		Entries: []EntityHistoryEntry{
			{
				ID:          1,
				ChangesetID: "cs-001",
				EntityID:    "REQ-001",
				Action:      "update",
				Before:      nil,
				After:       &after,
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
		},
		Count: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"entity_id", "entries", "count"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(out["entries"], &entries); err != nil {
		t.Fatalf("Unmarshal entries failed: %v", err)
	}
	for _, key := range []string{"id", "changeset_id", "entity_id", "action", "before", "after", "created_at"} {
		if _, ok := entries[0][key]; !ok {
			t.Errorf("expected entries[0] %q key", key)
		}
	}
}

func TestRelationHistoryResponseJSON(t *testing.T) {
	before := json.RawMessage(`{"from":"REQ-001","to":"DEC-001"}`)
	resp := RelationHistoryResponse{
		RelationKey: "REQ-001->DEC-001",
		Entries: []RelationHistoryEntry{
			{
				ID:          1,
				ChangesetID: "cs-001",
				RelationKey: "REQ-001->DEC-001",
				Action:      "delete",
				Before:      &before,
				After:       nil,
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
		},
		Count: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"relation_key", "entries", "count"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(out["entries"], &entries); err != nil {
		t.Fatalf("Unmarshal entries failed: %v", err)
	}
	for _, key := range []string{"id", "changeset_id", "relation_key", "action", "before", "after", "created_at"} {
		if _, ok := entries[0][key]; !ok {
			t.Errorf("expected entries[0] %q key", key)
		}
	}
}

func TestBootstrapScanResponseJSON(t *testing.T) {
	resp := BootstrapScanResponse{
		Entities: []BootstrapEntityCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Auth", Confidence: 0.9, Source: "file.md"},
		},
		Relations: []BootstrapRelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.8, Source: "file.md"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"entities", "relations"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var entities []map[string]json.RawMessage
	if err := json.Unmarshal(out["entities"], &entities); err != nil {
		t.Fatalf("Unmarshal entities failed: %v", err)
	}
	for _, key := range []string{"id", "type", "title", "confidence", "source"} {
		if _, ok := entities[0][key]; !ok {
			t.Errorf("expected entities[0] %q key", key)
		}
	}

	var relations []map[string]json.RawMessage
	if err := json.Unmarshal(out["relations"], &relations); err != nil {
		t.Fatalf("Unmarshal relations failed: %v", err)
	}
	for _, key := range []string{"from", "to", "type", "confidence", "source"} {
		if _, ok := relations[0][key]; !ok {
			t.Errorf("expected relations[0] %q key", key)
		}
	}
}

func TestBootstrapImportResponseJSON(t *testing.T) {
	resp := BootstrapImportResponse{
		Created: []string{"REQ-001", "DEC-001"},
		Skipped: []BootstrapSkippedItem{
			{ID: "REQ-002", Reason: "already exists"},
		},
		Errors: []BootstrapErrorItem{
			{ID: "REQ-003", Error: "invalid type"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	for _, key := range []string{"created", "skipped", "errors"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected top-level %q key", key)
		}
	}

	var skipped []map[string]json.RawMessage
	if err := json.Unmarshal(out["skipped"], &skipped); err != nil {
		t.Fatalf("Unmarshal skipped failed: %v", err)
	}
	for _, key := range []string{"id", "reason"} {
		if _, ok := skipped[0][key]; !ok {
			t.Errorf("expected skipped[0] %q key", key)
		}
	}

	var errors []map[string]json.RawMessage
	if err := json.Unmarshal(out["errors"], &errors); err != nil {
		t.Fatalf("Unmarshal errors failed: %v", err)
	}
	for _, key := range []string{"id", "error"} {
		if _, ok := errors[0][key]; !ok {
			t.Errorf("expected errors[0] %q key", key)
		}
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

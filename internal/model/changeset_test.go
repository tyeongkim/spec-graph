package model

import (
	"encoding/json"
	"testing"
)

func TestHistoryActionConstants(t *testing.T) {
	tests := []struct {
		action HistoryAction
		want   string
	}{
		{ActionCreate, "create"},
		{ActionUpdate, "update"},
		{ActionDeprecate, "deprecate"},
		{ActionDelete, "delete"},
	}
	for _, tt := range tests {
		if string(tt.action) != tt.want {
			t.Errorf("HistoryAction %q = %q; want %q", tt.action, string(tt.action), tt.want)
		}
	}
}

func TestChangesetJSONMarshal(t *testing.T) {
	cs := Changeset{
		ID:        "cs-001",
		Reason:    "initial setup",
		Actor:     "user@example.com",
		Source:    "cli",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal Changeset: %v", err)
	}

	var got Changeset
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal Changeset: %v", err)
	}

	if got.ID != cs.ID {
		t.Errorf("ID = %q; want %q", got.ID, cs.ID)
	}
	if got.Reason != cs.Reason {
		t.Errorf("Reason = %q; want %q", got.Reason, cs.Reason)
	}
	if got.Actor != cs.Actor {
		t.Errorf("Actor = %q; want %q", got.Actor, cs.Actor)
	}
	if got.Source != cs.Source {
		t.Errorf("Source = %q; want %q", got.Source, cs.Source)
	}
}

func TestChangesetOmitsEmptyOptionalFields(t *testing.T) {
	cs := Changeset{
		ID:        "cs-002",
		Reason:    "test",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal Changeset: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, ok := raw["actor"]; ok {
		t.Error("actor should be omitted when empty")
	}
	if _, ok := raw["source"]; ok {
		t.Error("source should be omitted when empty")
	}
}

func TestEntityHistoryEntryJSONMarshal_CreateCase(t *testing.T) {
	after := `{"id":"REQ-001","type":"requirement"}`
	entry := EntityHistoryEntry{
		ID:          1,
		ChangesetID: "cs-001",
		EntityID:    "REQ-001",
		Action:      ActionCreate,
		BeforeJSON:  nil,
		AfterJSON:   &after,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal EntityHistoryEntry: %v", err)
	}

	var got EntityHistoryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal EntityHistoryEntry: %v", err)
	}

	if got.Action != ActionCreate {
		t.Errorf("Action = %q; want %q", got.Action, ActionCreate)
	}
	if got.BeforeJSON != nil {
		t.Errorf("BeforeJSON = %v; want nil", got.BeforeJSON)
	}
	if got.AfterJSON == nil || *got.AfterJSON != after {
		t.Errorf("AfterJSON = %v; want %q", got.AfterJSON, after)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := raw["before_json"]; ok {
		t.Error("before_json should be omitted when nil")
	}
}

func TestRelationHistoryEntryJSONMarshal(t *testing.T) {
	before := `{"from_id":"REQ-001","to_id":"DEC-001","type":"depends_on"}`
	entry := RelationHistoryEntry{
		ID:          1,
		ChangesetID: "cs-001",
		RelationKey: "REQ-001:depends_on:DEC-001",
		Action:      ActionDelete,
		BeforeJSON:  &before,
		AfterJSON:   nil,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal RelationHistoryEntry: %v", err)
	}

	var got RelationHistoryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal RelationHistoryEntry: %v", err)
	}

	if got.Action != ActionDelete {
		t.Errorf("Action = %q; want %q", got.Action, ActionDelete)
	}
	if got.BeforeJSON == nil || *got.BeforeJSON != before {
		t.Errorf("BeforeJSON = %v; want %q", got.BeforeJSON, before)
	}
	if got.AfterJSON != nil {
		t.Errorf("AfterJSON = %v; want nil", got.AfterJSON)
	}
}

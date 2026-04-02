package store

import (
	"database/sql"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func insertTestChangeset(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec("INSERT INTO changesets (id, reason) VALUES (?, ?)", id, "test")
	if err != nil {
		t.Fatalf("insert test changeset: %v", err)
	}
}

func recordEntityChange(t *testing.T, tx *sql.Tx, hs *HistoryStore, entry model.EntityHistoryEntry) {
	t.Helper()
	if err := hs.RecordEntityChange(tx, entry); err != nil {
		t.Fatalf("record entity change: %v", err)
	}
}

func recordRelationChange(t *testing.T, tx *sql.Tx, hs *HistoryStore, entry model.RelationHistoryEntry) {
	t.Helper()
	if err := hs.RecordRelationChange(tx, entry); err != nil {
		t.Fatalf("record relation change: %v", err)
	}
}

func TestHistoryStore_RecordAndGetEntityHistory(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)
	insertTestChangeset(t, d, "cs-1")

	afterJSON := `{"id":"REQ-1","title":"Hello"}`

	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	recordEntityChange(t, tx, hs, model.EntityHistoryEntry{
		ChangesetID: "cs-1",
		EntityID:    "REQ-1",
		Action:      model.ActionCreate,
		AfterJSON:   &afterJSON,
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	entries, err := hs.GetEntityHistory("REQ-1")
	if err != nil {
		t.Fatalf("GetEntityHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d; want 1", len(entries))
	}

	e := entries[0]
	if e.ChangesetID != "cs-1" {
		t.Errorf("ChangesetID = %q; want %q", e.ChangesetID, "cs-1")
	}
	if e.EntityID != "REQ-1" {
		t.Errorf("EntityID = %q; want %q", e.EntityID, "REQ-1")
	}
	if e.Action != model.ActionCreate {
		t.Errorf("Action = %q; want %q", e.Action, model.ActionCreate)
	}
	if e.BeforeJSON != nil {
		t.Errorf("BeforeJSON = %v; want nil", e.BeforeJSON)
	}
	if e.AfterJSON == nil || *e.AfterJSON != afterJSON {
		t.Errorf("AfterJSON = %v; want %q", e.AfterJSON, afterJSON)
	}
	if e.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
}

func TestHistoryStore_RecordAndGetRelationHistory(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)
	insertTestChangeset(t, d, "cs-1")

	afterJSON := `{"from":"REQ-1","to":"TST-1","type":"verifies"}`

	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	recordRelationChange(t, tx, hs, model.RelationHistoryEntry{
		ChangesetID: "cs-1",
		RelationKey: "REQ-1->TST-1:verifies",
		Action:      model.ActionCreate,
		AfterJSON:   &afterJSON,
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	entries, err := hs.GetRelationHistory("REQ-1->TST-1:verifies")
	if err != nil {
		t.Fatalf("GetRelationHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d; want 1", len(entries))
	}

	e := entries[0]
	if e.ChangesetID != "cs-1" {
		t.Errorf("ChangesetID = %q; want %q", e.ChangesetID, "cs-1")
	}
	if e.RelationKey != "REQ-1->TST-1:verifies" {
		t.Errorf("RelationKey = %q; want %q", e.RelationKey, "REQ-1->TST-1:verifies")
	}
	if e.Action != model.ActionCreate {
		t.Errorf("Action = %q; want %q", e.Action, model.ActionCreate)
	}
	if e.AfterJSON == nil || *e.AfterJSON != afterJSON {
		t.Errorf("AfterJSON = %v; want %q", e.AfterJSON, afterJSON)
	}
}

func TestHistoryStore_GetChangesetHistory(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)
	insertTestChangeset(t, d, "cs-1")

	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	recordEntityChange(t, tx, hs, model.EntityHistoryEntry{
		ChangesetID: "cs-1",
		EntityID:    "REQ-1",
		Action:      model.ActionCreate,
		AfterJSON:   strPtr(`{"id":"REQ-1"}`),
	})
	recordEntityChange(t, tx, hs, model.EntityHistoryEntry{
		ChangesetID: "cs-1",
		EntityID:    "DEC-1",
		Action:      model.ActionCreate,
		AfterJSON:   strPtr(`{"id":"DEC-1"}`),
	})
	recordRelationChange(t, tx, hs, model.RelationHistoryEntry{
		ChangesetID: "cs-1",
		RelationKey: "REQ-1->DEC-1:informs",
		Action:      model.ActionCreate,
		AfterJSON:   strPtr(`{"from":"REQ-1","to":"DEC-1"}`),
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	entityEntries, relationEntries, err := hs.GetChangesetHistory("cs-1")
	if err != nil {
		t.Fatalf("GetChangesetHistory: %v", err)
	}
	if len(entityEntries) != 2 {
		t.Errorf("entity entries len = %d; want 2", len(entityEntries))
	}
	if len(relationEntries) != 1 {
		t.Errorf("relation entries len = %d; want 1", len(relationEntries))
	}
}

func TestHistoryStore_EmptyHistory(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)

	entries, err := hs.GetEntityHistory("REQ-999")
	if err != nil {
		t.Fatalf("GetEntityHistory: %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Errorf("len = %d; want 0", len(entries))
	}
}

func TestHistoryStore_NullBeforeJSON(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)
	insertTestChangeset(t, d, "cs-1")

	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	recordEntityChange(t, tx, hs, model.EntityHistoryEntry{
		ChangesetID: "cs-1",
		EntityID:    "REQ-1",
		Action:      model.ActionCreate,
		BeforeJSON:  nil,
		AfterJSON:   strPtr(`{"id":"REQ-1"}`),
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	entries, err := hs.GetEntityHistory("REQ-1")
	if err != nil {
		t.Fatalf("GetEntityHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d; want 1", len(entries))
	}
	if entries[0].BeforeJSON != nil {
		t.Errorf("BeforeJSON = %v; want nil", entries[0].BeforeJSON)
	}
	if entries[0].AfterJSON == nil {
		t.Error("AfterJSON is nil; want non-nil")
	}
}

func TestHistoryStore_MultipleEntries(t *testing.T) {
	d := setupTestDB(t)
	hs := NewHistoryStore(d)
	insertTestChangeset(t, d, "cs-1")
	insertTestChangeset(t, d, "cs-2")
	insertTestChangeset(t, d, "cs-3")

	for i, csID := range []string{"cs-1", "cs-2", "cs-3"} {
		tx, err := d.Begin()
		if err != nil {
			t.Fatalf("begin tx %d: %v", i, err)
		}
		recordEntityChange(t, tx, hs, model.EntityHistoryEntry{
			ChangesetID: csID,
			EntityID:    "REQ-1",
			Action:      model.ActionUpdate,
			BeforeJSON:  strPtr(`{"v":` + string(rune('0'+i)) + `}`),
			AfterJSON:   strPtr(`{"v":` + string(rune('1'+i)) + `}`),
		})
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	entries, err := hs.GetEntityHistory("REQ-1")
	if err != nil {
		t.Fatalf("GetEntityHistory: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d; want 3", len(entries))
	}

	for i := 0; i < len(entries)-1; i++ {
		if entries[i].CreatedAt < entries[i+1].CreatedAt {
			t.Errorf("entries not in DESC order: [%d].CreatedAt=%q < [%d].CreatedAt=%q",
				i, entries[i].CreatedAt, i+1, entries[i+1].CreatedAt)
		}
	}
}

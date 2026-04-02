package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/db"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.OpenMemoryDB()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func newTestEntityStore(t *testing.T, d *sql.DB) (*EntityStore, *ChangesetStore, *HistoryStore) {
	t.Helper()
	cs := NewChangesetStore(d)
	hs := NewHistoryStore(d)
	return NewEntityStore(d, cs, hs), cs, hs
}

func seedEntity(t *testing.T, s *EntityStore, id string, typ model.EntityType) model.Entity {
	t.Helper()
	e, err := s.Create(model.Entity{
		ID:    id,
		Type:  typ,
		Title: "Seed " + id,
	}, "", "", "")
	if err != nil {
		t.Fatalf("seed entity %s: %v", id, err)
	}
	return e
}

func seedRelation(t *testing.T, d *sql.DB, fromID, toID, relType string) {
	t.Helper()
	_, err := d.Exec(
		`INSERT INTO relations (from_id, to_id, type) VALUES (?, ?, ?)`,
		fromID, toID, relType,
	)
	if err != nil {
		t.Fatalf("seed relation %s->%s (%s): %v", fromID, toID, relType, err)
	}
}

func TestEntityStore_Create(t *testing.T) {
	allTypes := []struct {
		id  string
		typ model.EntityType
	}{
		{"REQ-1", model.EntityTypeRequirement},
		{"DEC-1", model.EntityTypeDecision},
		{"PHS-1", model.EntityTypePhase},
		{"API-1", model.EntityTypeInterface},
		{"STT-1", model.EntityTypeState},
		{"TST-1", model.EntityTypeTest},
		{"XCT-1", model.EntityTypeCrosscut},
		{"QST-1", model.EntityTypeQuestion},
		{"ASM-1", model.EntityTypeAssumption},
		{"ACT-1", model.EntityTypeCriterion},
		{"RSK-1", model.EntityTypeRisk},
	}

	t.Run("valid_all_types", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		for _, tc := range allTypes {
			t.Run(string(tc.typ), func(t *testing.T) {
				e, err := s.Create(model.Entity{
					ID:    tc.id,
					Type:  tc.typ,
					Title: "Title for " + tc.id,
				}, "", "", "")
				if err != nil {
					t.Fatalf("Create(%s) error: %v", tc.id, err)
				}
				if e.ID != tc.id {
					t.Errorf("ID = %q; want %q", e.ID, tc.id)
				}
				if e.Type != tc.typ {
					t.Errorf("Type = %q; want %q", e.Type, tc.typ)
				}
				if e.Status != model.EntityStatusDraft {
					t.Errorf("Status = %q; want %q", e.Status, model.EntityStatusDraft)
				}
				if string(e.Metadata) != "{}" {
					t.Errorf("Metadata = %s; want {}", e.Metadata)
				}
				if e.CreatedAt == "" {
					t.Error("CreatedAt is empty")
				}
				if e.UpdatedAt == "" {
					t.Error("UpdatedAt is empty")
				}
			})
		}
	})

	t.Run("duplicate_id", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		_, err := s.Create(model.Entity{
			ID:    "REQ-1",
			Type:  model.EntityTypeRequirement,
			Title: "Duplicate",
		}, "", "", "")
		var dup *model.ErrDuplicateEntity
		if !errors.As(err, &dup) {
			t.Fatalf("expected ErrDuplicateEntity, got %v", err)
		}
	})

	t.Run("invalid_type", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		_, err := s.Create(model.Entity{
			ID:    "FOO-1",
			Type:  model.EntityType("bogus"),
			Title: "Bad type",
		}, "", "", "")
		var inv *model.ErrInvalidInput
		if !errors.As(err, &inv) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("bad_id_format", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		_, err := s.Create(model.Entity{
			ID:    "req-1",
			Type:  model.EntityTypeRequirement,
			Title: "Bad format",
		}, "", "", "")
		var inv *model.ErrInvalidInput
		if !errors.As(err, &inv) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("prefix_mismatch", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		_, err := s.Create(model.Entity{
			ID:    "DEC-1",
			Type:  model.EntityTypeRequirement,
			Title: "Mismatch",
		}, "", "", "")
		var inv *model.ErrInvalidInput
		if !errors.As(err, &inv) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("default_status_draft", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		e, err := s.Create(model.Entity{
			ID:    "REQ-1",
			Type:  model.EntityTypeRequirement,
			Title: "No status",
		}, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.Status != model.EntityStatusDraft {
			t.Errorf("Status = %q; want %q", e.Status, model.EntityStatusDraft)
		}
	})

	t.Run("default_metadata_empty_json", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		e, err := s.Create(model.Entity{
			ID:    "REQ-1",
			Type:  model.EntityTypeRequirement,
			Title: "No metadata",
		}, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(e.Metadata) != "{}" {
			t.Errorf("Metadata = %s; want {}", e.Metadata)
		}
	})

	t.Run("empty_description_ok", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		e, err := s.Create(model.Entity{
			ID:    "REQ-1",
			Type:  model.EntityTypeRequirement,
			Title: "No desc",
		}, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.Description != "" {
			t.Errorf("Description = %q; want empty", e.Description)
		}
	})

	t.Run("custom_metadata_preserved", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		meta := json.RawMessage(`{"priority":"high"}`)
		e, err := s.Create(model.Entity{
			ID:       "REQ-1",
			Type:     model.EntityTypeRequirement,
			Title:    "With meta",
			Metadata: meta,
		}, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(e.Metadata) != `{"priority":"high"}` {
			t.Errorf("Metadata = %s; want %s", e.Metadata, meta)
		}
	})

	t.Run("timestamps_set", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		e, err := s.Create(model.Entity{
			ID:    "REQ-1",
			Type:  model.EntityTypeRequirement,
			Title: "Timestamps",
		}, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.CreatedAt == "" || e.UpdatedAt == "" {
			t.Errorf("timestamps not set: created=%q updated=%q", e.CreatedAt, e.UpdatedAt)
		}
	})
}

func TestEntityStore_Get(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		created := seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		got, err := s.Get("REQ-1")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID = %q; want %q", got.ID, created.ID)
		}
		if got.Type != created.Type {
			t.Errorf("Type = %q; want %q", got.Type, created.Type)
		}
		if got.Title != created.Title {
			t.Errorf("Title = %q; want %q", got.Title, created.Title)
		}
		if got.Status != created.Status {
			t.Errorf("Status = %q; want %q", got.Status, created.Status)
		}
		if string(got.Metadata) != string(created.Metadata) {
			t.Errorf("Metadata = %s; want %s", got.Metadata, created.Metadata)
		}
		if got.CreatedAt != created.CreatedAt {
			t.Errorf("CreatedAt = %q; want %q", got.CreatedAt, created.CreatedAt)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		_, err := s.Get("REQ-999")
		var nf *model.ErrEntityNotFound
		if !errors.As(err, &nf) {
			t.Fatalf("expected ErrEntityNotFound, got %v", err)
		}
	})
}

func TestEntityStore_List(t *testing.T) {
	t.Run("empty_db", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		entities, count, err := s.List(EntityFilters{})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if entities == nil {
			t.Fatal("expected empty slice, got nil")
		}
		if len(entities) != 0 {
			t.Errorf("len = %d; want 0", len(entities))
		}
		if count != 0 {
			t.Errorf("count = %d; want 0", count)
		}
	})

	t.Run("all_no_filters", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
		seedEntity(t, s, "DEC-1", model.EntityTypeDecision)
		seedEntity(t, s, "TST-1", model.EntityTypeTest)

		entities, count, err := s.List(EntityFilters{})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if len(entities) != 3 {
			t.Errorf("len = %d; want 3", len(entities))
		}
		if count != 3 {
			t.Errorf("count = %d; want 3", count)
		}
	})

	t.Run("filter_by_type", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
		seedEntity(t, s, "REQ-2", model.EntityTypeRequirement)
		seedEntity(t, s, "DEC-1", model.EntityTypeDecision)

		typ := model.EntityTypeRequirement
		entities, count, err := s.List(EntityFilters{Type: &typ})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if len(entities) != 2 {
			t.Errorf("len = %d; want 2", len(entities))
		}
		if count != 2 {
			t.Errorf("count = %d; want 2", count)
		}
	})

	t.Run("filter_by_status", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		active := model.EntityStatusActive
		_, err := s.Update("REQ-1", UpdateFields{Status: &active}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}

		seedEntity(t, s, "REQ-2", model.EntityTypeRequirement)

		status := model.EntityStatusActive
		entities, count, err := s.List(EntityFilters{Status: &status})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if len(entities) != 1 {
			t.Errorf("len = %d; want 1", len(entities))
		}
		if count != 1 {
			t.Errorf("count = %d; want 1", count)
		}
		if len(entities) > 0 && entities[0].ID != "REQ-1" {
			t.Errorf("ID = %q; want REQ-1", entities[0].ID)
		}
	})

	t.Run("filter_by_type_and_status", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
		seedEntity(t, s, "REQ-2", model.EntityTypeRequirement)
		seedEntity(t, s, "DEC-1", model.EntityTypeDecision)

		active := model.EntityStatusActive
		_, _ = s.Update("REQ-1", UpdateFields{Status: &active}, "", "", "", model.ActionUpdate)
		_, _ = s.Update("DEC-1", UpdateFields{Status: &active}, "", "", "", model.ActionUpdate)

		typ := model.EntityTypeRequirement
		entities, count, err := s.List(EntityFilters{Type: &typ, Status: &active})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if len(entities) != 1 {
			t.Errorf("len = %d; want 1", len(entities))
		}
		if count != 1 {
			t.Errorf("count = %d; want 1", count)
		}
	})
}

func TestEntityStore_Update(t *testing.T) {
	t.Run("title_only", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		newTitle := "Updated Title"
		got, err := s.Update("REQ-1", UpdateFields{Title: &newTitle}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}
		if got.Title != newTitle {
			t.Errorf("Title = %q; want %q", got.Title, newTitle)
		}
	})

	t.Run("description", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		desc := "New description"
		got, err := s.Update("REQ-1", UpdateFields{Description: &desc}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}
		if got.Description != desc {
			t.Errorf("Description = %q; want %q", got.Description, desc)
		}
	})

	t.Run("status", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		active := model.EntityStatusActive
		got, err := s.Update("REQ-1", UpdateFields{Status: &active}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}
		if got.Status != active {
			t.Errorf("Status = %q; want %q", got.Status, active)
		}
	})

	t.Run("metadata_full_replace", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		_, err := s.Create(model.Entity{
			ID:       "REQ-1",
			Type:     model.EntityTypeRequirement,
			Title:    "Meta test",
			Metadata: json.RawMessage(`{"a":"1","b":"2"}`),
		}, "", "", "")
		if err != nil {
			t.Fatalf("Create error: %v", err)
		}

		newMeta := json.RawMessage(`{"c":"3"}`)
		got, err := s.Update("REQ-1", UpdateFields{Metadata: &newMeta}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}
		if string(got.Metadata) != `{"c":"3"}` {
			t.Errorf("Metadata = %s; want %s", got.Metadata, `{"c":"3"}`)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		title := "Nope"
		_, err := s.Update("REQ-999", UpdateFields{Title: &title}, "", "", "", model.ActionUpdate)
		var nf *model.ErrEntityNotFound
		if !errors.As(err, &nf) {
			t.Fatalf("expected ErrEntityNotFound, got %v", err)
		}
	})

	t.Run("updated_at_changes", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		_, _ = d.Exec(`UPDATE entities SET created_at = datetime('now', '-1 minute'), updated_at = datetime('now', '-1 minute') WHERE id = ?`, "REQ-1")

		before, _ := s.Get("REQ-1")

		title := "Changed"
		got, err := s.Update("REQ-1", UpdateFields{Title: &title}, "", "", "", model.ActionUpdate)
		if err != nil {
			t.Fatalf("Update error: %v", err)
		}
		if got.UpdatedAt == before.UpdatedAt {
			t.Errorf("UpdatedAt did not change: %q", got.UpdatedAt)
		}
	})
}

func TestEntityStore_Delete(t *testing.T) {
	t.Run("existing_no_relations", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

		err := s.Delete("REQ-1", "", "", "")
		if err != nil {
			t.Fatalf("Delete error: %v", err)
		}

		_, err = s.Get("REQ-1")
		var nf *model.ErrEntityNotFound
		if !errors.As(err, &nf) {
			t.Fatalf("expected ErrEntityNotFound after delete, got %v", err)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)

		err := s.Delete("REQ-999", "", "", "")
		var nf *model.ErrEntityNotFound
		if !errors.As(err, &nf) {
			t.Fatalf("expected ErrEntityNotFound, got %v", err)
		}
	})

	t.Run("with_relations_blocked", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
		seedEntity(t, s, "TST-1", model.EntityTypeTest)

		seedRelation(t, d, "TST-1", "REQ-1", "verifies")

		err := s.Delete("REQ-1", "", "", "")
		var inv *model.ErrInvalidInput
		if !errors.As(err, &inv) {
			t.Fatalf("expected ErrInvalidInput for entity with relations, got %v", err)
		}
	})

	t.Run("with_outgoing_relation_blocked", func(t *testing.T) {
		d := setupTestDB(t)
		s, _, _ := newTestEntityStore(t, d)
		seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
		seedEntity(t, s, "TST-1", model.EntityTypeTest)

		seedRelation(t, d, "REQ-1", "TST-1", "references")

		err := s.Delete("REQ-1", "", "", "")
		var inv *model.ErrInvalidInput
		if !errors.As(err, &inv) {
			t.Fatalf("expected ErrInvalidInput for entity with outgoing relations, got %v", err)
		}
	})
}

func TestEntityStore_CreateRecordsChangeset(t *testing.T) {
	d := setupTestDB(t)
	s, cs, _ := newTestEntityStore(t, d)

	seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

	changesets, err := cs.List()
	if err != nil {
		t.Fatalf("list changesets: %v", err)
	}
	if len(changesets) != 1 {
		t.Fatalf("expected 1 changeset, got %d", len(changesets))
	}
	if changesets[0].ID != "CHG-1" {
		t.Errorf("changeset ID = %q; want CHG-1", changesets[0].ID)
	}
}

func TestEntityStore_UpdateRecordsHistory(t *testing.T) {
	d := setupTestDB(t)
	s, _, hs := newTestEntityStore(t, d)

	seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

	newTitle := "Updated"
	_, err := s.Update("REQ-1", UpdateFields{Title: &newTitle}, "title change", "", "", model.ActionUpdate)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	entries, err := hs.GetEntityHistory("REQ-1")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (create+update), got %d", len(entries))
	}

	var updateEntry model.EntityHistoryEntry
	found := false
	for _, e := range entries {
		if e.Action == model.ActionUpdate {
			updateEntry = e
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no update history entry found")
	}
	if updateEntry.BeforeJSON == nil {
		t.Fatal("before_json is nil")
	}
	if updateEntry.AfterJSON == nil {
		t.Fatal("after_json is nil")
	}

	var beforeEntity, afterEntity model.Entity
	if err := json.Unmarshal([]byte(*updateEntry.BeforeJSON), &beforeEntity); err != nil {
		t.Fatalf("unmarshal before: %v", err)
	}
	if err := json.Unmarshal([]byte(*updateEntry.AfterJSON), &afterEntity); err != nil {
		t.Fatalf("unmarshal after: %v", err)
	}
	if beforeEntity.Title == afterEntity.Title {
		t.Errorf("before and after titles should differ: %q", beforeEntity.Title)
	}
	if afterEntity.Title != "Updated" {
		t.Errorf("after title = %q; want %q", afterEntity.Title, "Updated")
	}
}

func TestEntityStore_DeleteRecordsHistory(t *testing.T) {
	d := setupTestDB(t)
	s, _, hs := newTestEntityStore(t, d)

	seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

	err := s.Delete("REQ-1", "removing", "", "")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	entries, err := hs.GetEntityHistory("REQ-1")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (create+delete), got %d", len(entries))
	}

	var deleteEntry model.EntityHistoryEntry
	found := false
	for _, e := range entries {
		if e.Action == model.ActionDelete {
			deleteEntry = e
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no delete history entry found")
	}
	if deleteEntry.BeforeJSON == nil {
		t.Fatal("before_json is nil for delete")
	}
	if deleteEntry.AfterJSON != nil {
		t.Errorf("after_json should be nil for delete, got %q", *deleteEntry.AfterJSON)
	}
}

func TestEntityStore_CreateSetsLayer(t *testing.T) {
	tests := []struct {
		id        string
		typ       model.EntityType
		wantLayer model.Layer
	}{
		{"REQ-1", model.EntityTypeRequirement, model.LayerArch},
		{"DEC-1", model.EntityTypeDecision, model.LayerArch},
		{"PHS-1", model.EntityTypePhase, model.LayerExec},
		{"API-1", model.EntityTypeInterface, model.LayerArch},
		{"TST-1", model.EntityTypeTest, model.LayerArch},
	}

	d := setupTestDB(t)
	s, _, _ := newTestEntityStore(t, d)

	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			e, err := s.Create(model.Entity{
				ID:    tt.id,
				Type:  tt.typ,
				Title: "Layer test " + tt.id,
			}, "", "", "")
			if err != nil {
				t.Fatalf("Create(%s) error: %v", tt.id, err)
			}
			if e.Layer != tt.wantLayer {
				t.Errorf("Layer = %q; want %q", e.Layer, tt.wantLayer)
			}
		})
	}
}

func TestEntityStore_ListFilterByLayer(t *testing.T) {
	d := setupTestDB(t)
	s, _, _ := newTestEntityStore(t, d)

	seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)
	seedEntity(t, s, "DEC-1", model.EntityTypeDecision)
	seedEntity(t, s, "PHS-1", model.EntityTypePhase)

	t.Run("filter_arch", func(t *testing.T) {
		layer := model.LayerArch
		entities, count, err := s.List(EntityFilters{Layer: &layer})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d; want 2", count)
		}
		if len(entities) != 2 {
			t.Errorf("len = %d; want 2", len(entities))
		}
	})

	t.Run("filter_exec", func(t *testing.T) {
		layer := model.LayerExec
		entities, count, err := s.List(EntityFilters{Layer: &layer})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d; want 1", count)
		}
		if len(entities) > 0 && entities[0].ID != "PHS-1" {
			t.Errorf("ID = %q; want PHS-1", entities[0].ID)
		}
	})

	t.Run("filter_mapping_empty", func(t *testing.T) {
		layer := model.LayerMapping
		entities, count, err := s.List(EntityFilters{Layer: &layer})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d; want 0", count)
		}
		if entities == nil {
			t.Fatal("expected empty slice, got nil")
		}
	})

	t.Run("filter_layer_and_type", func(t *testing.T) {
		layer := model.LayerArch
		typ := model.EntityTypeRequirement
		entities, count, err := s.List(EntityFilters{Layer: &layer, Type: &typ})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d; want 1", count)
		}
		if len(entities) > 0 && entities[0].ID != "REQ-1" {
			t.Errorf("ID = %q; want REQ-1", entities[0].ID)
		}
	})
}

func TestEntityStore_TransactionRollback(t *testing.T) {
	d := setupTestDB(t)
	s, cs, _ := newTestEntityStore(t, d)

	seedEntity(t, s, "REQ-1", model.EntityTypeRequirement)

	_, err := s.Create(model.Entity{
		ID:    "REQ-1",
		Type:  model.EntityTypeRequirement,
		Title: "Duplicate",
	}, "", "", "")
	if err == nil {
		t.Fatal("expected error for duplicate")
	}

	changesets, err := cs.List()
	if err != nil {
		t.Fatalf("list changesets: %v", err)
	}
	if len(changesets) != 1 {
		t.Errorf("expected 1 changeset (only from first create), got %d", len(changesets))
	}
}

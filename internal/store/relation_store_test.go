package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/taeyeong/spec-graph/internal/db"
	"github.com/taeyeong/spec-graph/internal/model"
)

func setupRelationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenMemoryDB()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

// createTestEntity inserts an entity directly via SQL (no entity_store dependency).
func createTestEntity(t *testing.T, database *sql.DB, id string, entityType model.EntityType) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO entities (id, type, title, status, metadata) VALUES (?, ?, ?, 'draft', '{}')`,
		id, string(entityType), "Test "+id,
	)
	if err != nil {
		t.Fatalf("create test entity %q: %v", id, err)
	}
}

func TestRelationStore_CreateRecordsChangeset(t *testing.T) {
	database := setupRelationTestDB(t)
	cs := NewChangesetStore(database)
	hs := NewHistoryStore(database)
	store := NewRelationStore(database, cs, hs)

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	rel, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "test reason", "test-actor", "cli")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Verify changeset was created.
	changesets, err := cs.List()
	if err != nil {
		t.Fatalf("list changesets: %v", err)
	}
	if len(changesets) != 1 {
		t.Fatalf("expected 1 changeset, got %d", len(changesets))
	}
	if changesets[0].Reason != "test reason" {
		t.Errorf("reason = %q; want %q", changesets[0].Reason, "test reason")
	}
	if changesets[0].Actor != "test-actor" {
		t.Errorf("actor = %q; want %q", changesets[0].Actor, "test-actor")
	}
	if changesets[0].Source != "cli" {
		t.Errorf("source = %q; want %q", changesets[0].Source, "cli")
	}

	// Verify relation history was recorded.
	relationKey := fmt.Sprintf("%s:%s:%s", rel.FromID, rel.ToID, rel.Type)
	entries, err := hs.GetRelationHistory(relationKey)
	if err != nil {
		t.Fatalf("get relation history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].Action != model.ActionCreate {
		t.Errorf("action = %q; want %q", entries[0].Action, model.ActionCreate)
	}
	if entries[0].AfterJSON == nil {
		t.Fatal("expected after_json to be set")
	}

	// Verify after_json contains the relation data.
	var recorded model.Relation
	if err := json.Unmarshal([]byte(*entries[0].AfterJSON), &recorded); err != nil {
		t.Fatalf("unmarshal after_json: %v", err)
	}
	if recorded.FromID != "REQ-1" {
		t.Errorf("recorded from_id = %q; want %q", recorded.FromID, "REQ-1")
	}
	if recorded.ToID != "DEC-1" {
		t.Errorf("recorded to_id = %q; want %q", recorded.ToID, "DEC-1")
	}
}

func TestRelationStore_DeleteRecordsHistory(t *testing.T) {
	database := setupRelationTestDB(t)
	cs := NewChangesetStore(database)
	hs := NewHistoryStore(database)
	store := NewRelationStore(database, cs, hs)

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "create reason", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = store.Delete("REQ-1", "DEC-1", model.RelationDependsOn, "delete reason", "deleter", "")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Should have 2 changesets (create + delete).
	changesets, err := cs.List()
	if err != nil {
		t.Fatalf("list changesets: %v", err)
	}
	if len(changesets) != 2 {
		t.Fatalf("expected 2 changesets, got %d", len(changesets))
	}

	// Verify relation history has both create and delete entries.
	relationKey := "REQ-1:DEC-1:depends_on"
	entries, err := hs.GetRelationHistory(relationKey)
	if err != nil {
		t.Fatalf("get relation history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}

	// Entries are ordered by created_at DESC; find delete entry by action.
	var deleteEntry model.RelationHistoryEntry
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
		t.Fatal("expected before_json to be set on delete")
	}

	var beforeRel model.Relation
	if err := json.Unmarshal([]byte(*deleteEntry.BeforeJSON), &beforeRel); err != nil {
		t.Fatalf("unmarshal before_json: %v", err)
	}
	if beforeRel.FromID != "REQ-1" {
		t.Errorf("before from_id = %q; want %q", beforeRel.FromID, "REQ-1")
	}
}

func TestRelationStore_InvalidRelationNoChangeset(t *testing.T) {
	database := setupRelationTestDB(t)
	cs := NewChangesetStore(database)
	hs := NewHistoryStore(database)
	store := NewRelationStore(database, cs, hs)

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "TST-1", model.EntityTypeTest)

	// This is an invalid edge (requirement→test for implements).
	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "TST-1",
		Type:   model.RelationImplements,
	}, "should not persist", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// No changeset should have been created.
	changesets, err := cs.List()
	if err != nil {
		t.Fatalf("list changesets: %v", err)
	}
	if len(changesets) != 0 {
		t.Errorf("expected 0 changesets after invalid edge, got %d", len(changesets))
	}
}

func strPtr(s string) *string                             { return &s }
func relTypePtr(r model.RelationType) *model.RelationType { return &r }

func TestRelationStore_Create_ValidEdges(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "REQ-2", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)
	createTestEntity(t, database, "API-1", model.EntityTypeInterface)
	createTestEntity(t, database, "STT-1", model.EntityTypeState)
	createTestEntity(t, database, "TST-1", model.EntityTypeTest)
	createTestEntity(t, database, "XCT-1", model.EntityTypeCrosscut)
	createTestEntity(t, database, "QST-1", model.EntityTypeQuestion)
	createTestEntity(t, database, "ASM-1", model.EntityTypeAssumption)
	createTestEntity(t, database, "ACT-1", model.EntityTypeCriterion)
	createTestEntity(t, database, "RSK-1", model.EntityTypeRisk)

	tests := []struct {
		name    string
		fromID  string
		toID    string
		relType model.RelationType
	}{
		{"implements: interface→requirement", "API-1", "REQ-1", model.RelationImplements},
		{"verifies: test→requirement", "TST-1", "REQ-1", model.RelationVerifies},
		{"depends_on: requirement→decision", "REQ-1", "DEC-1", model.RelationDependsOn},
		{"constrained_by: requirement→crosscut", "REQ-1", "XCT-1", model.RelationConstrainedBy},
		{"covers: phase→requirement", "PHS-1", "REQ-1", model.RelationCovers},
		{"delivers: phase→interface", "PHS-1", "API-1", model.RelationDelivers},
		{"triggers: interface→state", "API-1", "STT-1", model.RelationTriggers},
		{"answers: decision→question", "DEC-1", "QST-1", model.RelationAnswers},
		{"assumes: requirement→assumption", "REQ-1", "ASM-1", model.RelationAssumes},
		{"has_criterion: requirement→criterion", "REQ-1", "ACT-1", model.RelationHasCriterion},
		{"mitigates: decision→risk", "DEC-1", "RSK-1", model.RelationMitigates},
		{"supersedes: requirement→requirement (same type)", "REQ-1", "REQ-2", model.RelationSupersedes},
		{"conflicts_with: requirement→decision (any pair)", "REQ-1", "DEC-1", model.RelationConflictsWith},
		{"references: test→phase (any pair)", "TST-1", "PHS-1", model.RelationReferences},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, err := store.Create(model.Relation{
				FromID: tt.fromID,
				ToID:   tt.toID,
				Type:   tt.relType,
			}, "", "", "")
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if rel.ID == 0 {
				t.Error("expected non-zero ID")
			}
			if rel.FromID != tt.fromID {
				t.Errorf("from_id = %q; want %q", rel.FromID, tt.fromID)
			}
			if rel.ToID != tt.toID {
				t.Errorf("to_id = %q; want %q", rel.ToID, tt.toID)
			}
			if rel.Type != tt.relType {
				t.Errorf("type = %q; want %q", rel.Type, tt.relType)
			}
			if rel.CreatedAt == "" {
				t.Error("expected created_at to be set")
			}
		})
	}
}

func TestRelationStore_Create_InvalidEdges(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "REQ-2", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)
	createTestEntity(t, database, "API-1", model.EntityTypeInterface)
	createTestEntity(t, database, "STT-1", model.EntityTypeState)
	createTestEntity(t, database, "TST-1", model.EntityTypeTest)
	createTestEntity(t, database, "XCT-1", model.EntityTypeCrosscut)
	createTestEntity(t, database, "QST-1", model.EntityTypeQuestion)
	createTestEntity(t, database, "ASM-1", model.EntityTypeAssumption)
	createTestEntity(t, database, "ACT-1", model.EntityTypeCriterion)
	createTestEntity(t, database, "RSK-1", model.EntityTypeRisk)

	tests := []struct {
		name    string
		fromID  string
		toID    string
		relType model.RelationType
	}{
		{"implements: requirement→requirement ✗", "REQ-1", "REQ-2", model.RelationImplements},
		{"verifies: requirement→test ✗", "REQ-1", "TST-1", model.RelationVerifies},
		{"depends_on: test→phase ✗ (phase not in to)", "TST-1", "PHS-1", model.RelationDependsOn},
		{"constrained_by: test→crosscut ✗ (test not in from)", "TST-1", "XCT-1", model.RelationConstrainedBy},
		{"covers: crosscut→phase ✗", "XCT-1", "PHS-1", model.RelationCovers},
		{"delivers: requirement→phase ✗", "REQ-1", "PHS-1", model.RelationDelivers},
		{"triggers: test→state ✗", "TST-1", "STT-1", model.RelationTriggers},
		{"answers: requirement→question ✗", "REQ-1", "QST-1", model.RelationAnswers},
		{"assumes: test→assumption ✗", "TST-1", "ASM-1", model.RelationAssumes},
		{"has_criterion: decision→criterion ✗", "DEC-1", "ACT-1", model.RelationHasCriterion},
		{"mitigates: requirement→risk ✗", "REQ-1", "RSK-1", model.RelationMitigates},
		{"supersedes: requirement→decision ✗ (different types)", "REQ-1", "DEC-1", model.RelationSupersedes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Create(model.Relation{
				FromID: tt.fromID,
				ToID:   tt.toID,
				Type:   tt.relType,
			}, "", "", "")
			if err == nil {
				t.Fatal("expected ErrInvalidEdge, got nil")
			}
			var target *model.ErrInvalidEdge
			if !errors.As(err, &target) {
				t.Errorf("expected ErrInvalidEdge, got: %T: %v", err, err)
			}
		})
	}
}

func TestRelationStore_Create_EntityNotFound(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)

	tests := []struct {
		name   string
		fromID string
		toID   string
		wantID string // which ID should be in the error
	}{
		{"from_id not found", "REQ-999", "REQ-1", "REQ-999"},
		{"to_id not found", "REQ-1", "REQ-999", "REQ-999"},
		{"both not found (from checked first)", "REQ-888", "REQ-999", "REQ-888"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Create(model.Relation{
				FromID: tt.fromID,
				ToID:   tt.toID,
				Type:   model.RelationDependsOn,
			}, "", "", "")
			if err == nil {
				t.Fatal("expected ErrEntityNotFound, got nil")
			}
			var target *model.ErrEntityNotFound
			if !errors.As(err, &target) {
				t.Errorf("expected ErrEntityNotFound, got: %T: %v", err, err)
			}
			if target.ID != tt.wantID {
				t.Errorf("error ID = %q; want %q", target.ID, tt.wantID)
			}
		})
	}
}

func TestRelationStore_Create_SelfLoop(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)

	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "REQ-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err == nil {
		t.Fatal("expected ErrSelfLoop, got nil")
	}
	var target *model.ErrSelfLoop
	if !errors.As(err, &target) {
		t.Errorf("expected ErrSelfLoop, got: %T: %v", err, err)
	}
	if target.ID != "REQ-1" {
		t.Errorf("error ID = %q; want %q", target.ID, "REQ-1")
	}
}

func TestRelationStore_Create_Duplicate(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	// First create should succeed.
	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Duplicate should fail.
	_, err = store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err == nil {
		t.Fatal("expected ErrDuplicateRelation, got nil")
	}
	var target *model.ErrDuplicateRelation
	if !errors.As(err, &target) {
		t.Errorf("expected ErrDuplicateRelation, got: %T: %v", err, err)
	}
}

func TestRelationStore_Create_DefaultWeightAndMetadata(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	rel, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rel.Weight != 1.0 {
		t.Errorf("weight = %f; want 1.0", rel.Weight)
	}
	if string(rel.Metadata) != "{}" {
		t.Errorf("metadata = %s; want {}", string(rel.Metadata))
	}
}

func TestRelationStore_Create_CustomWeightAndMetadata(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	rel, err := store.Create(model.Relation{
		FromID:   "REQ-1",
		ToID:     "DEC-1",
		Type:     model.RelationDependsOn,
		Weight:   0.5,
		Metadata: []byte(`{"reason":"critical"}`),
	}, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rel.Weight != 0.5 {
		t.Errorf("weight = %f; want 0.5", rel.Weight)
	}
	if string(rel.Metadata) != `{"reason":"critical"}` {
		t.Errorf("metadata = %s; want {\"reason\":\"critical\"}", string(rel.Metadata))
	}
}

func TestRelationStore_Create_ValidationOrder(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	// Only create one entity so we can test order.
	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)

	// from_id not found is checked before self-loop.
	_, err := store.Create(model.Relation{
		FromID: "NOPE-1",
		ToID:   "NOPE-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	var notFound *model.ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrEntityNotFound for from_id check first, got: %T: %v", err, err)
	}

	// to_id not found is checked before self-loop (from exists, to doesn't).
	_, err = store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "NOPE-2",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrEntityNotFound for to_id check, got: %T: %v", err, err)
	}
}

func TestRelationStore_List(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "REQ-2", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)

	// Create several relations.
	mustCreate := func(from, to string, rt model.RelationType) {
		t.Helper()
		_, err := store.Create(model.Relation{FromID: from, ToID: to, Type: rt}, "", "", "")
		if err != nil {
			t.Fatalf("create %s→%s (%s): %v", from, to, rt, err)
		}
	}
	mustCreate("REQ-1", "DEC-1", model.RelationDependsOn)
	mustCreate("REQ-2", "DEC-1", model.RelationDependsOn)
	mustCreate("PHS-1", "REQ-1", model.RelationCovers)

	tests := []struct {
		name      string
		filters   RelationFilters
		wantCount int
	}{
		{"no filters → all", RelationFilters{}, 3},
		{"by from_id REQ-1", RelationFilters{FromID: strPtr("REQ-1")}, 1},
		{"by from_id REQ-2", RelationFilters{FromID: strPtr("REQ-2")}, 1},
		{"by from_id PHS-1", RelationFilters{FromID: strPtr("PHS-1")}, 1},
		{"by to_id DEC-1", RelationFilters{ToID: strPtr("DEC-1")}, 2},
		{"by to_id REQ-1", RelationFilters{ToID: strPtr("REQ-1")}, 1},
		{"by type depends_on", RelationFilters{Type: relTypePtr(model.RelationDependsOn)}, 2},
		{"by type covers", RelationFilters{Type: relTypePtr(model.RelationCovers)}, 1},
		{"combined from+type", RelationFilters{FromID: strPtr("REQ-1"), Type: relTypePtr(model.RelationDependsOn)}, 1},
		{"combined from+to", RelationFilters{FromID: strPtr("REQ-1"), ToID: strPtr("DEC-1")}, 1},
		{"no match", RelationFilters{FromID: strPtr("DEC-1")}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rels, count, err := store.List(tt.filters)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("count = %d; want %d", count, tt.wantCount)
			}
			if len(rels) != tt.wantCount {
				t.Errorf("len(rels) = %d; want %d", len(rels), tt.wantCount)
			}
			// Empty result must be empty slice, not nil.
			if tt.wantCount == 0 && rels == nil {
				t.Error("expected empty slice, got nil")
			}
		})
	}
}

func TestRelationStore_Delete(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("delete existing", func(t *testing.T) {
		err := store.Delete("REQ-1", "DEC-1", model.RelationDependsOn, "", "", "")
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		// Verify gone.
		rels, count, _ := store.List(RelationFilters{FromID: strPtr("REQ-1")})
		if count != 0 {
			t.Errorf("count after delete = %d; want 0", count)
		}
		if len(rels) != 0 {
			t.Errorf("len(rels) after delete = %d; want 0", len(rels))
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := store.Delete("REQ-1", "DEC-1", model.RelationDependsOn, "", "", "")
		if err == nil {
			t.Fatal("expected ErrRelationNotFound, got nil")
		}
		var target *model.ErrRelationNotFound
		if !errors.As(err, &target) {
			t.Errorf("expected ErrRelationNotFound, got: %T: %v", err, err)
		}
	})
}

func TestRelationStore_HasRelations(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)

	t.Run("no relations", func(t *testing.T) {
		has, err := store.HasRelations("REQ-1")
		if err != nil {
			t.Fatalf("has relations: %v", err)
		}
		if has {
			t.Error("expected false, got true")
		}
	})

	// Create a relation REQ-1 → DEC-1.
	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("has relation as from", func(t *testing.T) {
		has, err := store.HasRelations("REQ-1")
		if err != nil {
			t.Fatalf("has relations: %v", err)
		}
		if !has {
			t.Error("expected true, got false")
		}
	})

	t.Run("has relation as to", func(t *testing.T) {
		has, err := store.HasRelations("DEC-1")
		if err != nil {
			t.Fatalf("has relations: %v", err)
		}
		if !has {
			t.Error("expected true, got false")
		}
	})

	t.Run("no relation for unrelated entity", func(t *testing.T) {
		has, err := store.HasRelations("PHS-1")
		if err != nil {
			t.Fatalf("has relations: %v", err)
		}
		if has {
			t.Error("expected false, got true")
		}
	})
}

func TestRelationStore_Create_SameFromToDifferentTypes(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)

	// Two different relation types between same entities should both succeed.
	_, err := store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationDependsOn,
	}, "", "", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = store.Create(model.Relation{
		FromID: "REQ-1",
		ToID:   "DEC-1",
		Type:   model.RelationConflictsWith,
	}, "", "", "")
	if err != nil {
		t.Fatalf("second create (different type): %v", err)
	}
}

func layerPtr(l model.Layer) *model.Layer { return &l }

func TestRelationStore_CreateSetsLayer(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)
	createTestEntity(t, database, "API-1", model.EntityTypeInterface)

	tests := []struct {
		name      string
		fromID    string
		toID      string
		relType   model.RelationType
		wantLayer model.Layer
	}{
		{"arch: depends_on", "REQ-1", "DEC-1", model.RelationDependsOn, model.LayerArch},
		{"mapping: covers", "PHS-1", "REQ-1", model.RelationCovers, model.LayerMapping},
		{"mapping: delivers", "PHS-1", "API-1", model.RelationDelivers, model.LayerMapping},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, err := store.Create(model.Relation{
				FromID: tt.fromID,
				ToID:   tt.toID,
				Type:   tt.relType,
			}, "", "", "")
			if err != nil {
				t.Fatalf("Create error: %v", err)
			}
			if rel.Layer != tt.wantLayer {
				t.Errorf("Layer = %q; want %q", rel.Layer, tt.wantLayer)
			}
		})
	}
}

func TestRelationStore_ListFilterByLayer(t *testing.T) {
	database := setupRelationTestDB(t)
	store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))

	createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
	createTestEntity(t, database, "REQ-2", model.EntityTypeRequirement)
	createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
	createTestEntity(t, database, "PHS-1", model.EntityTypePhase)

	mustCreate := func(from, to string, rt model.RelationType) {
		t.Helper()
		_, err := store.Create(model.Relation{FromID: from, ToID: to, Type: rt}, "", "", "")
		if err != nil {
			t.Fatalf("create %s→%s (%s): %v", from, to, rt, err)
		}
	}
	mustCreate("REQ-1", "DEC-1", model.RelationDependsOn)
	mustCreate("REQ-2", "DEC-1", model.RelationDependsOn)
	mustCreate("PHS-1", "REQ-1", model.RelationCovers)

	t.Run("filter_arch", func(t *testing.T) {
		rels, count, err := store.List(RelationFilters{Layer: layerPtr(model.LayerArch)})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d; want 2", count)
		}
		if len(rels) != 2 {
			t.Errorf("len = %d; want 2", len(rels))
		}
	})

	t.Run("filter_mapping", func(t *testing.T) {
		rels, count, err := store.List(RelationFilters{Layer: layerPtr(model.LayerMapping)})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d; want 1", count)
		}
		if len(rels) > 0 && rels[0].Type != model.RelationCovers {
			t.Errorf("Type = %q; want %q", rels[0].Type, model.RelationCovers)
		}
	})

	t.Run("filter_exec_empty", func(t *testing.T) {
		rels, count, err := store.List(RelationFilters{Layer: layerPtr(model.LayerExec)})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d; want 0", count)
		}
		if rels == nil {
			t.Fatal("expected empty slice, got nil")
		}
	})

	t.Run("filter_layer_and_type", func(t *testing.T) {
		rels, count, err := store.List(RelationFilters{
			Layer: layerPtr(model.LayerArch),
			Type:  relTypePtr(model.RelationDependsOn),
		})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d; want 2", count)
		}
		if len(rels) != 2 {
			t.Errorf("len = %d; want 2", len(rels))
		}
	})
}

func TestRelationStore_GetByEntity(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, store *RelationStore, database *sql.DB)
		entityID  string
		wantCount int
	}{
		{
			name: "entity_as_from_only",
			setup: func(t *testing.T, store *RelationStore, database *sql.DB) {
				createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
				createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
				_, err := store.Create(model.Relation{FromID: "REQ-1", ToID: "DEC-1", Type: model.RelationDependsOn}, "", "", "")
				if err != nil {
					t.Fatalf("create: %v", err)
				}
			},
			entityID:  "REQ-1",
			wantCount: 1,
		},
		{
			name: "entity_as_to_only",
			setup: func(t *testing.T, store *RelationStore, database *sql.DB) {
				createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
				createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
				_, err := store.Create(model.Relation{FromID: "REQ-1", ToID: "DEC-1", Type: model.RelationDependsOn}, "", "", "")
				if err != nil {
					t.Fatalf("create: %v", err)
				}
			},
			entityID:  "DEC-1",
			wantCount: 1,
		},
		{
			name: "entity_as_both",
			setup: func(t *testing.T, store *RelationStore, database *sql.DB) {
				createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
				createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
				createTestEntity(t, database, "API-1", model.EntityTypeInterface)
				_, err := store.Create(model.Relation{FromID: "REQ-1", ToID: "DEC-1", Type: model.RelationDependsOn}, "", "", "")
				if err != nil {
					t.Fatalf("create depends_on: %v", err)
				}
				_, err = store.Create(model.Relation{FromID: "API-1", ToID: "REQ-1", Type: model.RelationImplements}, "", "", "")
				if err != nil {
					t.Fatalf("create implements: %v", err)
				}
			},
			entityID:  "REQ-1",
			wantCount: 2,
		},
		{
			name: "entity_with_no_relations",
			setup: func(t *testing.T, store *RelationStore, database *sql.DB) {
				createTestEntity(t, database, "PHS-1", model.EntityTypePhase)
			},
			entityID:  "PHS-1",
			wantCount: 0,
		},
		{
			name:      "entity_not_found",
			setup:     func(t *testing.T, store *RelationStore, database *sql.DB) {},
			entityID:  "NONEXIST-001",
			wantCount: 0,
		},
		{
			name: "multiple_relation_types",
			setup: func(t *testing.T, store *RelationStore, database *sql.DB) {
				createTestEntity(t, database, "REQ-1", model.EntityTypeRequirement)
				createTestEntity(t, database, "DEC-1", model.EntityTypeDecision)
				createTestEntity(t, database, "PHS-1", model.EntityTypePhase)
				_, err := store.Create(model.Relation{FromID: "REQ-1", ToID: "DEC-1", Type: model.RelationDependsOn}, "", "", "")
				if err != nil {
					t.Fatalf("create depends_on: %v", err)
				}
				_, err = store.Create(model.Relation{FromID: "PHS-1", ToID: "REQ-1", Type: model.RelationCovers}, "", "", "")
				if err != nil {
					t.Fatalf("create covers: %v", err)
				}
			},
			entityID:  "REQ-1",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := setupRelationTestDB(t)
			store := NewRelationStore(database, NewChangesetStore(database), NewHistoryStore(database))
			tt.setup(t, store, database)

			rels, err := store.GetByEntity(tt.entityID)
			if err != nil {
				t.Fatalf("GetByEntity: %v", err)
			}
			if len(rels) != tt.wantCount {
				t.Errorf("len(rels) = %d; want %d", len(rels), tt.wantCount)
			}
			if tt.wantCount == 0 && rels == nil {
				t.Error("expected empty slice, got nil")
			}
		})
	}
}

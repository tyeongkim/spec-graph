package specgraph_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestBelongsToRejectsCartesianPairs(t *testing.T) {
	tests := []struct {
		name     string
		fromType string
		fromID   string
		toType   string
		toID     string
	}{
		{name: "phase to phase", fromType: "phase", fromID: "PHS-001", toType: "phase", toID: "PHS-002"},
		{name: "task to plan", fromType: "task", fromID: "TSK-001", toType: "plan", toID: "PLN-001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eng := openTestEngine(t)
			ctx := context.Background()
			if test.fromType == "task" {
				createTask(t, eng, test.fromID)
			} else if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: test.fromType, ID: test.fromID, Title: test.fromID}); err != nil {
				t.Fatalf("create source: %v", err)
			}
			if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: test.toType, ID: test.toID, Title: test.toID}); err != nil {
				t.Fatalf("create target: %v", err)
			}
			_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: test.fromID, To: test.toID, Type: "belongs_to"})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
			if !strings.Contains(err.Error(), test.fromID) || !strings.Contains(err.Error(), test.toID) {
				t.Errorf("error %q does not name illegal pair %s -> %s", err, test.fromID, test.toID)
			}
		})
	}
}

func TestTaskRejectsSecondParent(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createTask(t, eng, "TSK-001")
	for _, phaseID := range []string{"PHS-001", "PHS-002"} {
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "phase", ID: phaseID, Title: phaseID}); err != nil {
			t.Fatalf("create %s: %v", phaseID, err)
		}
	}
	if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "TSK-001", To: "PHS-001", Type: "belongs_to"}); err != nil {
		t.Fatalf("first parent: %v", err)
	}
	_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "TSK-001", To: "PHS-002", Type: "belongs_to"})
	assertErrorCode(t, err, specgraph.CodeInvalidInput)
	for _, id := range []string{"PHS-001", "PHS-002"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("error %q does not name parent %s", err, id)
		}
	}
}

// setupRelationTestEntities creates two entities for relation testing:
// REQ-001 (requirement) and API-001 (interface). An interface→requirement
// "implements" edge between them is permitted by the edge matrix.
func setupRelationTestEntities(t *testing.T, eng *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()
	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type:  "requirement",
		ID:    "REQ-001",
		Title: "Test requirement",
	}); err != nil {
		t.Fatalf("create REQ-001: %v", err)
	}
	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type:  "interface",
		ID:    "API-001",
		Title: "Test interface",
	}); err != nil {
		t.Fatalf("create API-001: %v", err)
	}
}

func TestAddRelation(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		got, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		})
		if err != nil {
			t.Fatalf("AddRelation: %v", err)
		}
		if got.FromID != "API-001" {
			t.Errorf("FromID = %q, want %q", got.FromID, "API-001")
		}
		if got.ToID != "REQ-001" {
			t.Errorf("ToID = %q, want %q", got.ToID, "REQ-001")
		}
		if string(got.Type) != "implements" {
			t.Errorf("Type = %q, want %q", got.Type, "implements")
		}
		if string(got.Layer) != "arch" {
			t.Errorf("Layer = %q, want %q", got.Layer, "arch")
		}
	})

	t.Run("with weight", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		got, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From:   "API-001",
			To:     "REQ-001",
			Type:   "implements",
			Weight: 2.5,
		})
		if err != nil {
			t.Fatalf("AddRelation: %v", err)
		}
		if got.Weight != 2.5 {
			t.Errorf("Weight = %v, want %v", got.Weight, 2.5)
		}
	})

	t.Run("with metadata", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		got, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From:     "API-001",
			To:       "REQ-001",
			Type:     "implements",
			Metadata: json.RawMessage(`{"note":"primary"}`),
		})
		if err != nil {
			t.Fatalf("AddRelation: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(got.Metadata, &m); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if m["note"] != "primary" {
			t.Errorf("metadata note = %v, want %q", m["note"], "primary")
		}
	})

	t.Run("self-loop rejected", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "REQ-001",
			To:   "REQ-001",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("missing required fields", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		cases := []struct {
			name string
			req  specgraph.AddRelationRequest
		}{
			{
				name: "empty from",
				req:  specgraph.AddRelationRequest{From: "", To: "REQ-001", Type: "implements"},
			},
			{
				name: "empty to",
				req:  specgraph.AddRelationRequest{From: "API-001", To: "", Type: "implements"},
			},
			{
				name: "empty type",
				req:  specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: ""},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := eng.AddRelation(ctx, tc.req)
				assertErrorCode(t, err, specgraph.CodeInvalidInput)
			})
		}
	})

	t.Run("invalid relation type", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "not_a_real_type",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("from entity not found", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-999",
			To:   "REQ-001",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeNotFound)
	})

	t.Run("to entity not found", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-999",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeNotFound)
	})

	t.Run("invalid edge", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		// "implements" requires interface→requirement; the reverse
		// requirement→interface is not a permitted edge.
		_, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "REQ-001",
			To:   "API-001",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("duplicate relation", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		req := specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}
		if _, err := eng.AddRelation(ctx, req); err != nil {
			t.Fatalf("first AddRelation: %v", err)
		}

		_, err := eng.AddRelation(ctx, req)
		assertErrorCode(t, err, specgraph.CodeConflict)
	})

	t.Run("symmetric normalization", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		// conflicts_with is symmetric and any→any. Adding it from the
		// lexicographically larger ID (REQ-001) to the smaller (API-001)
		// must be normalized internally and succeed.
		got, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "REQ-001",
			To:   "API-001",
			Type: "conflicts_with",
		})
		if err != nil {
			t.Fatalf("AddRelation conflicts_with: %v", err)
		}
		if string(got.Type) != "conflicts_with" {
			t.Errorf("Type = %q, want %q", got.Type, "conflicts_with")
		}
	})

	t.Run("auto-activate on delivers", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()

		// A plan + phase that belongs to it, plus a draft requirement.
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "plan",
			ID:    "PLN-001",
			Title: "Test plan",
		}); err != nil {
			t.Fatalf("create PLN-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "phase",
			ID:    "PHS-001",
			Title: "Test phase",
		}); err != nil {
			t.Fatalf("create PHS-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-002",
			Title: "Delivered requirement",
		}); err != nil {
			t.Fatalf("create REQ-002: %v", err)
		}

		// Phases belong to a plan before mapping relations apply.
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "PHS-001",
			To:   "PLN-001",
			Type: "belongs_to",
		}); err != nil {
			t.Fatalf("AddRelation belongs_to: %v", err)
		}

		// REQ-002 starts as draft.
		before, err := eng.GetEntity(ctx, "REQ-002")
		if err != nil {
			t.Fatalf("GetEntity REQ-002 before: %v", err)
		}
		if string(before.Status) != "draft" {
			t.Fatalf("REQ-002 status = %q before delivers, want %q", before.Status, "draft")
		}

		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "PHS-001",
			To:   "REQ-002",
			Type: "delivers",
		}); err != nil {
			t.Fatalf("AddRelation delivers: %v", err)
		}

		after, err := eng.GetEntity(ctx, "REQ-002")
		if err != nil {
			t.Fatalf("GetEntity REQ-002 after: %v", err)
		}
		if string(after.Status) != "active" {
			t.Errorf("REQ-002 status = %q after delivers, want %q", after.Status, "active")
		}
	})
}

func TestListRelations(t *testing.T) {
	t.Parallel()

	t.Run("empty result", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if rels == nil {
			t.Error("expected non-nil slice, got nil")
		}
		if len(rels) != 0 {
			t.Errorf("len = %d, want 0", len(rels))
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("list all", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "test",
			ID:    "TST-001",
			Title: "Test case",
		}); err != nil {
			t.Fatalf("create TST-001: %v", err)
		}

		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "TST-001", To: "REQ-001", Type: "verifies"}); err != nil {
			t.Fatalf("AddRelation verifies: %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 2 {
			t.Fatalf("len = %d, want 2", len(rels))
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("filter by from", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "test",
			ID:    "TST-001",
			Title: "Test case",
		}); err != nil {
			t.Fatalf("create TST-001: %v", err)
		}

		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "TST-001", To: "REQ-001", Type: "verifies"}); err != nil {
			t.Fatalf("AddRelation verifies: %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{From: "API-001"})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 1 {
			t.Fatalf("len = %d, want 1", len(rels))
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if rels[0].FromID != "API-001" {
			t.Errorf("FromID = %q, want %q", rels[0].FromID, "API-001")
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "test",
			ID:    "TST-001",
			Title: "Test case",
		}); err != nil {
			t.Fatalf("create TST-001: %v", err)
		}

		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "TST-001", To: "REQ-001", Type: "verifies"}); err != nil {
			t.Fatalf("AddRelation verifies: %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{Type: "implements"})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 1 {
			t.Fatalf("len = %d, want 1", len(rels))
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if string(rels[0].Type) != "implements" {
			t.Errorf("Type = %q, want %q", rels[0].Type, "implements")
		}
	})

	t.Run("filter by layer", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		// One arch relation (implements) and one mapping relation (delivers).
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Plan"}); err != nil {
			t.Fatalf("create PLN-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"}); err != nil {
			t.Fatalf("create PHS-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-002", Title: "Delivered"}); err != nil {
			t.Fatalf("create REQ-002: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"}); err != nil {
			t.Fatalf("AddRelation belongs_to: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-001", To: "REQ-002", Type: "delivers"}); err != nil {
			t.Fatalf("AddRelation delivers: %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{Layer: "arch"})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 1 {
			t.Fatalf("len = %d, want 1", len(rels))
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if string(rels[0].Layer) != "arch" {
			t.Errorf("Layer = %q, want %q", rels[0].Layer, "arch")
		}
	})
}

func TestDeleteRelation(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}); err != nil {
			t.Fatalf("AddRelation: %v", err)
		}

		if err := eng.DeleteRelation(ctx, specgraph.DeleteRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}); err != nil {
			t.Fatalf("DeleteRelation: %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 0 {
			t.Errorf("len = %d, want 0", len(rels))
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		cases := []struct {
			name string
			req  specgraph.DeleteRelationRequest
		}{
			{
				name: "empty from",
				req:  specgraph.DeleteRelationRequest{From: "", To: "REQ-001", Type: "implements"},
			},
			{
				name: "empty to",
				req:  specgraph.DeleteRelationRequest{From: "API-001", To: "", Type: "implements"},
			},
			{
				name: "empty type",
				req:  specgraph.DeleteRelationRequest{From: "API-001", To: "REQ-001", Type: ""},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := eng.DeleteRelation(ctx, tc.req)
				assertErrorCode(t, err, specgraph.CodeInvalidInput)
			})
		}
	})

	t.Run("relation not found", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		// Owner entity exists, but the relation was never added.
		err := eng.DeleteRelation(ctx, specgraph.DeleteRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeNotFound)
	})

	t.Run("owner entity not found", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		err := eng.DeleteRelation(ctx, specgraph.DeleteRelationRequest{
			From: "API-999",
			To:   "REQ-001",
			Type: "implements",
		})
		assertErrorCode(t, err, specgraph.CodeNotFound)
	})

	t.Run("symmetric delete", func(t *testing.T) {
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)

		// Add conflicts_with API-001 -> REQ-001.
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "conflicts_with",
		}); err != nil {
			t.Fatalf("AddRelation conflicts_with: %v", err)
		}

		// Delete with reversed IDs; symmetric normalization must locate it.
		if err := eng.DeleteRelation(ctx, specgraph.DeleteRelationRequest{
			From: "REQ-001",
			To:   "API-001",
			Type: "conflicts_with",
		}); err != nil {
			t.Fatalf("DeleteRelation (reversed): %v", err)
		}

		rels, count, err := eng.ListRelations(ctx, specgraph.ListRelationsRequest{})
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		if len(rels) != 0 {
			t.Errorf("len = %d, want 0", len(rels))
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})
}

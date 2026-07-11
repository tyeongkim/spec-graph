package specgraph_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// openTestEngine opens an Engine rooted at a fresh, initialized temp directory
// and registers a cleanup that closes it. It fails the test on any open error.
func openTestEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := newInitializedRoot(t)
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

// assertErrorCode verifies that err is (or wraps) a *specgraph.Error whose Code
// equals the wanted code.
func assertErrorCode(t *testing.T, err error, code specgraph.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var sgErr *specgraph.Error
	if !errors.As(err, &sgErr) {
		t.Fatalf("expected *specgraph.Error, got %T: %v", err, err)
	}
	if sgErr.Code != code {
		t.Errorf("got code %q, want %q (message: %s)", sgErr.Code, code, sgErr.Message)
	}
}

// stringPtr returns a pointer to s, for populating optional update fields.
func stringPtr(s string) *string { return &s }

func TestCreateEntity(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:        "requirement",
			ID:          "REQ-001",
			Title:       "User authentication",
			Description: "Users can sign in",
		})
		if err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}
		if ent.ID != "REQ-001" {
			t.Errorf("ID = %q, want %q", ent.ID, "REQ-001")
		}
		if ent.Title != "User authentication" {
			t.Errorf("Title = %q, want %q", ent.Title, "User authentication")
		}
		if ent.Description != "Users can sign in" {
			t.Errorf("Description = %q, want %q", ent.Description, "Users can sign in")
		}
	})

	t.Run("default status", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "No status given",
		})
		if err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}
		if got := string(ent.Status); got != "draft" {
			t.Errorf("default Status = %q, want %q", got, "draft")
		}
	})

	t.Run("duplicate entity", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		req := specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "First",
		}
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("first CreateEntity: %v", err)
		}

		_, err := eng.CreateEntity(ctx, req)
		assertErrorCode(t, err, specgraph.CodeConflict)
	})

	t.Run("invalid ID format", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		// "DEC-001" carries the decision prefix but the type is requirement.
		_, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "DEC-001",
			Title: "Wrong prefix",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("missing required fields", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		cases := []struct {
			name string
			req  specgraph.CreateEntityRequest
		}{
			{
				name: "empty type",
				req:  specgraph.CreateEntityRequest{Type: "", ID: "REQ-001", Title: "T"},
			},
			{
				name: "empty title",
				req:  specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: ""},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := eng.CreateEntity(ctx, tc.req)
				assertErrorCode(t, err, specgraph.CodeInvalidInput)
			})
		}
	})

	t.Run("with metadata", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		meta := json.RawMessage(`{"priority":"high"}`)
		ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:     "requirement",
			ID:       "REQ-001",
			Title:    "With metadata",
			Metadata: meta,
		})
		if err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(ent.Metadata, &got); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got["priority"] != "high" {
			t.Errorf("metadata priority = %v, want %q", got["priority"], "high")
		}
	})
}

func TestCreateEntityAutoID(t *testing.T) {
	t.Parallel()

	createIDs := func(t *testing.T, eng *specgraph.Engine, ids ...string) {
		t.Helper()
		ctx := context.Background()
		for _, id := range ids {
			if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type:  "requirement",
				ID:    id,
				Title: "seed " + id,
			}); err != nil {
				t.Fatalf("seed CreateEntity %q: %v", id, err)
			}
		}
	}

	tests := []struct {
		name string
		seed []string
	}{
		{name: "empty graph", seed: nil},
		{name: "unpadded sequence", seed: []string{"REQ-1", "REQ-2"}},
		{name: "padded sequence", seed: []string{"REQ-001"}},
		{name: "mixed forms", seed: []string{"REQ-001", "REQ-5"}},
		{name: "gap unpadded", seed: []string{"REQ-1", "REQ-9"}},
		{name: "wide legacy number", seed: []string{"REQ-999"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			eng := openTestEngine(t)
			ctx := context.Background()
			createIDs(t, eng, tt.seed...)

			ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type:  "requirement",
				Title: "auto",
			})
			if err != nil {
				t.Fatalf("CreateEntity (auto): %v", err)
			}
			if err := model.ValidateEntityID(ent.ID, model.EntityTypeRequirement); err != nil {
				t.Errorf("auto ID %q failed validation: %v", ent.ID, err)
			}

			got, err := eng.GetEntity(ctx, ent.ID)
			if err != nil {
				t.Fatalf("GetEntity %q after auto-create: %v", ent.ID, err)
			}
			if got.ID != ent.ID {
				t.Errorf("persisted ID = %q, want %q", got.ID, ent.ID)
			}
		})
	}

	t.Run("distinct IDs across multiple creates", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		const n = 20
		seen := make(map[string]bool, n)
		for i := 0; i < n; i++ {
			ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
				Type:  "requirement",
				Title: "auto",
			})
			if err != nil {
				t.Fatalf("CreateEntity (auto) #%d: %v", i, err)
			}
			if seen[ent.ID] {
				t.Fatalf("duplicate auto ID generated: %q", ent.ID)
			}
			seen[ent.ID] = true
		}
	})

	t.Run("correct prefix per type", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		dec, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "decision",
			Title: "first decision",
		})
		if err != nil {
			t.Fatalf("CreateEntity decision: %v", err)
		}
		if err := model.ValidateEntityID(dec.ID, model.EntityTypeDecision); err != nil {
			t.Errorf("decision auto ID %q failed validation: %v", dec.ID, err)
		}
		prefix, _, _, ok := model.ParseEntityID(dec.ID)
		if !ok || prefix != "DEC" {
			t.Errorf("decision auto ID %q has prefix %q; want %q", dec.ID, prefix, "DEC")
		}
	})

	t.Run("explicit legacy id still works", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		ent, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "explicit legacy",
		})
		if err != nil {
			t.Fatalf("CreateEntity (explicit legacy): %v", err)
		}
		if ent.ID != "REQ-001" {
			t.Errorf("explicit ID = %q, want %q", ent.ID, "REQ-001")
		}
	})

	t.Run("unknown type errors", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "bogus",
			Title: "no prefix",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})
}

func TestGetEntity(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		created, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "Fetch me",
		})
		if err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}

		got, err := eng.GetEntity(ctx, "REQ-001")
		if err != nil {
			t.Fatalf("GetEntity: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID = %q, want %q", got.ID, created.ID)
		}
		if got.Title != created.Title {
			t.Errorf("Title = %q, want %q", got.Title, created.Title)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.GetEntity(ctx, "REQ-999")
		assertNotFound(t, err)
	})
}

func TestListEntities(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		ents, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{})
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if len(ents) != 0 {
			t.Errorf("len = %d, want 0", len(ents))
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "r1"}); err != nil {
			t.Fatalf("CreateEntity REQ-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-002", Title: "r2"}); err != nil {
			t.Fatalf("CreateEntity REQ-002: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "decision", ID: "DEC-001", Title: "d1"}); err != nil {
			t.Fatalf("CreateEntity DEC-001: %v", err)
		}

		ents, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{Type: "requirement"})
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if len(ents) != 2 {
			t.Fatalf("len = %d, want 2", len(ents))
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
		for _, e := range ents {
			if string(e.Type) != "requirement" {
				t.Errorf("got type %q, want requirement", e.Type)
			}
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "r1"}); err != nil {
			t.Fatalf("CreateEntity REQ-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-002", Title: "r2"}); err != nil {
			t.Fatalf("CreateEntity REQ-002: %v", err)
		}
		if _, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "REQ-002", Status: stringPtr("active")}); err != nil {
			t.Fatalf("UpdateEntity REQ-002: %v", err)
		}

		ents, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{Status: "active"})
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if len(ents) != 1 {
			t.Fatalf("len = %d, want 1", len(ents))
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if ents[0].ID != "REQ-002" {
			t.Errorf("ID = %q, want REQ-002", ents[0].ID)
		}
	})

	t.Run("no filter returns all", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "r1"}); err != nil {
			t.Fatalf("CreateEntity REQ-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "decision", ID: "DEC-001", Title: "d1"}); err != nil {
			t.Fatalf("CreateEntity DEC-001: %v", err)
		}

		ents, count, err := eng.ListEntities(ctx, specgraph.ListEntitiesRequest{})
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if len(ents) != 2 {
			t.Errorf("len = %d, want 2", len(ents))
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})
}

func TestUpdateEntity(t *testing.T) {
	t.Parallel()

	// seed creates a single requirement entity and returns the engine + ctx.
	seed := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:        "requirement",
			ID:          "REQ-001",
			Title:       "Original title",
			Description: "Original description",
		}); err != nil {
			t.Fatalf("seed CreateEntity: %v", err)
		}
		return eng, ctx
	}

	t.Run("update title", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seed(t)

		res, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:    "REQ-001",
			Title: stringPtr("New title"),
		})
		if err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}
		if res.Entity.Title != "New title" {
			t.Errorf("Title = %q, want %q", res.Entity.Title, "New title")
		}
	})

	t.Run("update description", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seed(t)

		res, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:          "REQ-001",
			Description: stringPtr("New description"),
		})
		if err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}
		if res.Entity.Description != "New description" {
			t.Errorf("Description = %q, want %q", res.Entity.Description, "New description")
		}
	})

	t.Run("update metadata", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seed(t)

		meta := json.RawMessage(`{"owner":"alice"}`)
		res, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:       "REQ-001",
			Metadata: &meta,
		})
		if err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(res.Entity.Metadata, &m); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if m["owner"] != "alice" {
			t.Errorf("metadata owner = %v, want %q", m["owner"], "alice")
		}
	})

	t.Run("update status", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seed(t)

		res, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:     "REQ-001",
			Status: stringPtr("active"),
		})
		if err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}
		if s := string(res.Entity.Status); s != "active" {
			t.Errorf("Status = %q, want %q", s, "active")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:    "REQ-999",
			Title: stringPtr("Nope"),
		})
		assertNotFound(t, err)
	})

	t.Run("nil fields unchanged", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seed(t)

		res, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:    "REQ-001",
			Title: stringPtr("Changed title only"),
		})
		if err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}
		if res.Entity.Title != "Changed title only" {
			t.Errorf("Title = %q, want %q", res.Entity.Title, "Changed title only")
		}
		if res.Entity.Description != "Original description" {
			t.Errorf("Description = %q, want %q (should be unchanged)", res.Entity.Description, "Original description")
		}
	})
}

func TestDeprecateEntity(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "To deprecate",
		}); err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}

		got, err := eng.DeprecateEntity(ctx, "REQ-001")
		if err != nil {
			t.Fatalf("DeprecateEntity: %v", err)
		}
		if s := string(got.Status); s != "deprecated" {
			t.Errorf("Status = %q, want %q", s, "deprecated")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.DeprecateEntity(ctx, "REQ-999")
		assertNotFound(t, err)
	})
}

func TestDeleteEntity(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "To delete",
		}); err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}

		if err := eng.DeleteEntity(ctx, "REQ-001"); err != nil {
			t.Fatalf("DeleteEntity: %v", err)
		}

		_, err := eng.GetEntity(ctx, "REQ-001")
		assertNotFound(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		err := eng.DeleteEntity(ctx, "REQ-999")
		assertNotFound(t, err)
	})

	// TODO: test delete with relations once relation CRUD is on Engine
}

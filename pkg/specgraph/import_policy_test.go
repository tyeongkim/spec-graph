package specgraph

import (
	"context"
	"testing"
)

func TestImportEntitiesCommitsAsOneUnit(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	result, err := eng.ImportEntities(context.Background(), ImportEntitiesRequest{
		Entities: []CreateEntityRequest{
			{Type: "requirement", ID: "REQ-001", Title: "First"},
			{Type: "requirement", ID: "REQ-002", Title: "Second"},
			{Type: "decision", ID: "DEC-001", Title: "Decision"},
		},
	})
	if err != nil {
		t.Fatalf("ImportEntities: %v", err)
	}
	if len(result.Created) != 3 {
		t.Errorf("Created = %v, want 3 entities", result.Created)
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped = %v, want none", result.Skipped)
	}

	if files := entityFileSnapshot(t, eng.Root()); len(files) != 3 {
		t.Errorf("entity file count = %d, want 3", len(files))
	}
}

func TestImportEntitiesSkipsExistingAndAbortsOnInvalid(t *testing.T) {
	t.Parallel()

	t.Run("existing entity is skipped", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()
		if _, err := eng.CreateEntity(ctx, CreateEntityRequest{
			Type: "requirement", ID: "REQ-001", Title: "Existing",
		}); err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}

		result, err := eng.ImportEntities(ctx, ImportEntitiesRequest{
			Entities: []CreateEntityRequest{
				{Type: "requirement", ID: "REQ-001", Title: "Duplicate"},
				{Type: "requirement", ID: "REQ-002", Title: "New"},
			},
		})
		if err != nil {
			t.Fatalf("ImportEntities: %v", err)
		}
		if len(result.Skipped) != 1 || result.Skipped[0].ID != "REQ-001" {
			t.Errorf("Skipped = %v, want REQ-001", result.Skipped)
		}
		if len(result.Created) != 1 || result.Created[0] != "REQ-002" {
			t.Errorf("Created = %v, want [REQ-002]", result.Created)
		}

		existing, err := eng.GetEntity(ctx, "REQ-001")
		if err != nil {
			t.Fatalf("GetEntity existing entity: %v", err)
		}
		if existing.Title != "Existing" {
			t.Errorf("existing entity title = %q, want %q", existing.Title, "Existing")
		}
	})

	t.Run("invalid item aborts the whole import", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)

		_, err := eng.ImportEntities(context.Background(), ImportEntitiesRequest{
			Entities: []CreateEntityRequest{
				{Type: "requirement", ID: "REQ-001", Title: "Valid"},
				{Type: "requirement", ID: "DEC-001", Title: "Prefix mismatch"},
			},
		})
		if err == nil {
			t.Fatal("ImportEntities error = nil, want the invalid item rejected")
		}
		if files := entityFileSnapshot(t, eng.Root()); len(files) != 0 {
			t.Errorf("entity file count = %d, want 0", len(files))
		}
	})
}

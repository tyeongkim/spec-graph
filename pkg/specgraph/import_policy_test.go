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

func TestBootstrapImportSkipDecisionsDoNotAbort(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()
	if _, err := eng.CreateEntity(ctx, CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "Existing",
	}); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	result, err := eng.BootstrapImport(ctx, BootstrapImportRequest{
		Entities: []BootstrapCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Duplicate", Confidence: 0.9},
			{ID: "REQ-002", Type: "requirement", Title: "Unsure", Confidence: 0.2},
			{ID: "API-001", Type: "interface", Title: "Accepted", Confidence: 0.9},
		},
		Relations: []BootstrapRelationCandidate{
			{From: "API-001", To: "REQ-001", Type: "implements", Confidence: 0.9},
			{From: "REQ-001", To: "API-001", Type: "implements", Confidence: 0.9},
		},
	})
	if err != nil {
		t.Fatalf("BootstrapImport: %v", err)
	}

	skipReasons := make(map[string]string, len(result.Skipped))
	for _, item := range result.Skipped {
		skipReasons[item.ID] = item.Reason
	}
	for id, want := range map[string]string{
		"REQ-001":                    "already exists",
		"REQ-002":                    "low confidence",
		"REQ-001:API-001:implements": "invalid edge",
	} {
		if got := skipReasons[id]; got != want {
			t.Errorf("skip reason for %q = %q, want %q", id, got, want)
		}
	}

	created := make(map[string]bool, len(result.Created))
	for _, id := range result.Created {
		created[id] = true
	}
	if !created["API-001"] {
		t.Errorf("Created = %v, want API-001", result.Created)
	}
	if !created["API-001:REQ-001:implements"] {
		t.Errorf("Created = %v, want the valid relation", result.Created)
	}
	if _, err := eng.GetEntity(ctx, "REQ-002"); !IsNotFound(err) {
		t.Errorf("GetEntity low-confidence candidate error = %v; want not found", err)
	}
}

func TestBootstrapImportAbortsOnInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  BootstrapImportRequest
	}{
		{
			name: "unknown entity type",
			req: BootstrapImportRequest{Entities: []BootstrapCandidate{
				{ID: "REQ-001", Type: "requirement", Title: "Valid", Confidence: 0.9},
				{ID: "XXX-001", Type: "not-a-type", Title: "Invalid", Confidence: 0.9},
			}},
		},
		{
			name: "low-confidence candidate is still validated",
			req: BootstrapImportRequest{Entities: []BootstrapCandidate{
				{ID: "XXX-001", Type: "not-a-type", Title: "Invalid", Confidence: 0.1},
			}},
		},
		{
			name: "unknown relation type",
			req: BootstrapImportRequest{Entities: []BootstrapCandidate{
				{ID: "REQ-001", Type: "requirement", Title: "Valid", Confidence: 0.9},
				{ID: "API-001", Type: "interface", Title: "Valid", Confidence: 0.9},
			}, Relations: []BootstrapRelationCandidate{
				{From: "API-001", To: "REQ-001", Type: "not-a-relation", Confidence: 0.9},
			}},
		},
		{
			name: "relation endpoint does not exist",
			req: BootstrapImportRequest{Entities: []BootstrapCandidate{
				{ID: "API-001", Type: "interface", Title: "Valid", Confidence: 0.9},
			}, Relations: []BootstrapRelationCandidate{
				{From: "API-001", To: "REQ-404", Type: "implements", Confidence: 0.9},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := openTestEngine(t)

			if _, err := eng.BootstrapImport(context.Background(), tc.req); err == nil {
				t.Fatal("BootstrapImport error = nil, want the invalid candidate rejected")
			}
			if files := entityFileSnapshot(t, eng.Root()); len(files) != 0 {
				t.Errorf("entity file count = %d, want 0", len(files))
			}
		})
	}
}

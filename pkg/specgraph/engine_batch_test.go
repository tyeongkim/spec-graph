package specgraph

import (
	"context"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestApplyBatchResolvesRefsToGeneratedIDs(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	result, err := eng.ApplyBatch(context.Background(), BatchRequest{
		Entities: []BatchEntity{
			{Ref: "plan", CreateEntityRequest: CreateEntityRequest{Type: "plan", Title: "v1 Delivery", Status: "active"}},
			{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "Phase 1"}},
		},
		Relations: []BatchRelation{
			{From: "phase", To: "plan", Type: "belongs_to"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	ids := make(map[string]string, len(result.Entities))
	for _, item := range result.Entities {
		ids[item.Ref] = item.Entity.ID
	}
	if ids["plan"] == "" || ids["phase"] == "" {
		t.Fatalf("Entities = %+v, want generated IDs for both refs", result.Entities)
	}
	if ids["plan"] == ids["phase"] {
		t.Errorf("plan and phase share ID %q, want distinct IDs", ids["plan"])
	}

	if len(result.Relations) != 1 {
		t.Fatalf("Relations = %+v, want one relation", result.Relations)
	}
	relation := result.Relations[0]
	if relation.FromID != ids["phase"] || relation.ToID != ids["plan"] {
		t.Errorf("relation = %s -> %s, want %s -> %s", relation.FromID, relation.ToID, ids["phase"], ids["plan"])
	}

	persisted, _, err := eng.ListRelations(context.Background(), ListRelationsRequest{})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(persisted) != 1 || persisted[0].FromID != ids["phase"] || persisted[0].ToID != ids["plan"] {
		t.Errorf("persisted relations = %+v, want %s -> %s", persisted, ids["phase"], ids["plan"])
	}
}

func TestApplyBatchMixesRefsWithExistingIDs(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()

	existing, err := eng.CreateEntity(ctx, CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "Payments must be idempotent",
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	result, err := eng.ApplyBatch(ctx, BatchRequest{
		Entities: []BatchEntity{
			{Ref: "criterion", CreateEntityRequest: CreateEntityRequest{
				Type:     "criterion",
				Title:    "Duplicate request processed once",
				Metadata: []byte(`{"given":"request sent","when":"resent","then":"no duplicate"}`),
			}},
		},
		Relations: []BatchRelation{
			{From: existing.ID, To: "criterion", Type: "has_criterion"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	criterionID := result.Entities[0].Entity.ID
	if got := result.Relations[0]; got.FromID != existing.ID || got.ToID != criterionID {
		t.Errorf("relation = %s -> %s, want %s -> %s", got.FromID, got.ToID, existing.ID, criterionID)
	}
}

func TestApplyBatchReferencesRefLessEntityBySuppliedID(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	result, err := eng.ApplyBatch(context.Background(), BatchRequest{
		Entities: []BatchEntity{
			{CreateEntityRequest: CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Plan", Status: "active"}},
			{CreateEntityRequest: CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"}},
		},
		Relations: []BatchRelation{
			{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if got := result.Relations[0]; got.FromID != "PHS-001" || got.ToID != "PLN-001" {
		t.Errorf("relation = %s -> %s, want PHS-001 -> PLN-001", got.FromID, got.ToID)
	}
}

func TestApplyBatchLeavesGraphUntouchedOnFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     BatchRequest
		wantMsg string
	}{
		{
			name: "invalid entity aborts the batch",
			req: BatchRequest{Entities: []BatchEntity{
				{Ref: "good", CreateEntityRequest: CreateEntityRequest{Type: "requirement", Title: "Valid"}},
				{Ref: "bad", CreateEntityRequest: CreateEntityRequest{Type: "requirement", ID: "DEC-001", Title: "Prefix mismatch"}},
			}},
			wantMsg: `entity 1 (ref "bad")`,
		},
		{
			name: "relation rejected by the edge matrix aborts the batch",
			req: BatchRequest{
				Entities: []BatchEntity{
					{Ref: "req", CreateEntityRequest: CreateEntityRequest{Type: "requirement", Title: "Requirement"}},
					{Ref: "plan", CreateEntityRequest: CreateEntityRequest{Type: "plan", Title: "Plan"}},
				},
				Relations: []BatchRelation{
					{From: "req", To: "plan", Type: "belongs_to"},
				},
			},
			wantMsg: "relation 0",
		},
		{
			name: "undeclared ref aborts the batch",
			req: BatchRequest{
				Entities: []BatchEntity{
					{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "Phase 1"}},
				},
				Relations: []BatchRelation{
					{From: "phase", To: "typo-plan", Type: "belongs_to"},
				},
			},
			wantMsg: `to "typo-plan" is neither a ref declared in this batch nor an entity ID`,
		},
		{
			name: "duplicate ref aborts the batch",
			req: BatchRequest{Entities: []BatchEntity{
				{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "First"}},
				{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "Second"}},
			}},
			wantMsg: `ref "phase" is declared more than once`,
		},
		{
			name: "ref shaped like an entity ID is rejected",
			req: BatchRequest{Entities: []BatchEntity{
				{Ref: "REQ-001", CreateEntityRequest: CreateEntityRequest{Type: "requirement", Title: "Confusing ref"}},
			}},
			wantMsg: `ref "REQ-001" must not look like an entity ID`,
		},
		{
			name:    "empty batch is rejected",
			req:     BatchRequest{},
			wantMsg: "batch must contain at least one entity or relation",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := openTestEngine(t)

			_, err := eng.ApplyBatch(context.Background(), tc.req)
			if err == nil {
				t.Fatal("ApplyBatch error = nil, want the batch rejected")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantMsg)
			}
			if files := entityFileSnapshot(t, eng.Root()); len(files) != 0 {
				t.Errorf("entity file count = %d, want 0", len(files))
			}
		})
	}
}

func TestApplyBatchActivatesDeliveredTarget(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	result, err := eng.ApplyBatch(context.Background(), BatchRequest{
		Entities: []BatchEntity{
			{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "Phase 1"}},
			{Ref: "req", CreateEntityRequest: CreateEntityRequest{Type: "requirement", Title: "Delivered requirement"}},
		},
		Relations: []BatchRelation{
			{From: "phase", To: "req", Type: "delivers"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	var requirementID string
	var reported model.Entity
	for _, item := range result.Entities {
		if item.Ref == "req" {
			requirementID = item.Entity.ID
			reported = item.Entity
		}
	}
	if reported.Status != model.EntityStatusActive {
		t.Errorf("reported requirement status = %q, want %q", reported.Status, model.EntityStatusActive)
	}

	delivered, err := eng.GetEntity(context.Background(), requirementID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if delivered.Status != model.EntityStatusActive {
		t.Errorf("delivered requirement status = %q, want %q", delivered.Status, model.EntityStatusActive)
	}
}

func TestApplyBatchReportsDefaultWeightAsStored(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()

	result, err := eng.ApplyBatch(ctx, BatchRequest{
		Entities: []BatchEntity{
			{Ref: "plan", CreateEntityRequest: CreateEntityRequest{Type: "plan", Title: "Plan", Status: "active"}},
			{Ref: "phase", CreateEntityRequest: CreateEntityRequest{Type: "phase", Title: "Phase"}},
		},
		Relations: []BatchRelation{
			{From: "phase", To: "plan", Type: "belongs_to"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if got := result.Relations[0].Weight; got != 1.0 {
		t.Errorf("reported weight = %v, want 1.0", got)
	}

	persisted, _, err := eng.ListRelations(ctx, ListRelationsRequest{})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if persisted[0].Weight != result.Relations[0].Weight {
		t.Errorf("persisted weight = %v, reported %v; want them equal", persisted[0].Weight, result.Relations[0].Weight)
	}
}

func TestApplyBatchGeneratesDistinctIDsForSameTypeEntities(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	entities := make([]BatchEntity, 0, 30)
	for range 30 {
		entities = append(entities, BatchEntity{
			CreateEntityRequest: CreateEntityRequest{
				Type:  "requirement",
				Title: "Requirement",
			},
		})
	}

	result, err := eng.ApplyBatch(context.Background(), BatchRequest{Entities: entities})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	seen := make(map[string]bool, len(result.Entities))
	for _, item := range result.Entities {
		if seen[item.Entity.ID] {
			t.Fatalf("duplicate generated ID %q within one batch", item.Entity.ID)
		}
		seen[item.Entity.ID] = true
	}
	if len(seen) != 30 {
		t.Errorf("distinct ID count = %d, want 30", len(seen))
	}
}

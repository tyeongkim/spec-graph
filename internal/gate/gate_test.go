package gate

import (
	"fmt"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type mockRelationFetcher struct {
	relations map[string][]model.Relation
}

func (m *mockRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	return m.relations[entityID], nil
}

type mockEntityFetcher struct {
	entities map[string]model.Entity
}

func (m *mockEntityFetcher) Get(id string) (model.Entity, error) {
	e, ok := m.entities[id]
	if !ok {
		return model.Entity{}, fmt.Errorf("entity %q not found", id)
	}
	return e, nil
}

func (m *mockEntityFetcher) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	var result []model.Entity
	for _, e := range m.entities {
		if filters.Type != nil && e.Type != *filters.Type {
			continue
		}
		if filters.Status != nil && e.Status != *filters.Status {
			continue
		}
		if filters.Layer != nil && e.Layer != *filters.Layer {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}

func archEntity(id string, typ model.EntityType, status model.EntityStatus) model.Entity {
	return model.Entity{
		ID:     id,
		Type:   typ,
		Layer:  model.LayerArch,
		Title:  id,
		Status: status,
	}
}

func execEntity(id string, typ model.EntityType, status model.EntityStatus) model.Entity {
	return model.Entity{
		ID:     id,
		Type:   typ,
		Layer:  model.LayerExec,
		Title:  id,
		Status: status,
	}
}

func rel(fromID, toID string, relType model.RelationType) model.Relation {
	return model.Relation{
		FromID: fromID,
		ToID:   toID,
		Type:   relType,
		Layer:  model.LayerForRelationType(relType),
	}
}

func TestLookupPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		target     Target
		wantNil    bool
		wantChecks []string
	}{
		{
			name: "phase to resolved returns delivery_completeness and gates",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			wantNil:    false,
			wantChecks: []string{"delivery_completeness", "gates"},
		},
		{
			name: "plan to resolved returns plan_coverage",
			target: Target{
				EntityID:   "PLN-001",
				EntityType: model.EntityTypePlan,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			wantNil:    false,
			wantChecks: []string{"plan_coverage"},
		},
		{
			name: "phase to active returns nil (not gated)",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusDraft,
				ToStatus:   model.EntityStatusActive,
			},
			wantNil: true,
		},
		{
			name: "requirement to resolved returns nil",
			target: Target{
				EntityID:   "REQ-001",
				EntityType: model.EntityTypeRequirement,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			wantNil: true,
		},
		{
			name: "phase to deprecated returns nil",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusDeprecated,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := LookupPolicy(tt.target)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil policy, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil policy, got nil")
			}

			if len(got.Checks) != len(tt.wantChecks) {
				t.Fatalf("checks count: got %d, want %d", len(got.Checks), len(tt.wantChecks))
			}
			for i, c := range got.Checks {
				if c != tt.wantChecks[i] {
					t.Errorf("check[%d]: got %q, want %q", i, c, tt.wantChecks[i])
				}
			}

			if len(got.BlockingSeverities) != 2 {
				t.Errorf("blocking severities: got %d, want 2", len(got.BlockingSeverities))
			}
		})
	}
}

func TestEnforce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		target            Target
		entities          map[string]model.Entity
		relations         map[string][]model.Relation
		wantBlocked       bool
		wantBlockingCount int
		wantWarningCount  int
		wantErr           bool
	}{
		{
			name: "non-gated transition returns empty report",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusDraft,
				ToStatus:   model.EntityStatusActive,
			},
			entities:          map[string]model.Entity{},
			relations:         map[string][]model.Relation{},
			wantBlocked:       false,
			wantBlockingCount: 0,
			wantWarningCount:  0,
		},
		{
			name: "phase with all gates satisfied not blocked",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
				"REQ-001": archEntity("REQ-001", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {
					rel("PHS-001", "REQ-001", model.RelationCovers),
					rel("PHS-001", "REQ-001", model.RelationDelivers),
				},
				"REQ-001": {
					rel("PHS-001", "REQ-001", model.RelationCovers),
					rel("PHS-001", "REQ-001", model.RelationDelivers),
				},
			},
			wantBlocked:       false,
			wantBlockingCount: 0,
			wantWarningCount:  0,
		},
		{
			name: "phase with unresolved question is blocked",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
				"QST-001": archEntity("QST-001", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {
					rel("PHS-001", "QST-001", model.RelationCovers),
				},
				"QST-001": {
					rel("PHS-001", "QST-001", model.RelationCovers),
				},
			},
			wantBlocked:       true,
			wantBlockingCount: 1,
			wantWarningCount:  0,
		},
		{
			name: "phase with unmitigated risk is blocked",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
				"RSK-001": archEntity("RSK-001", model.EntityTypeRisk, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {
					rel("PHS-001", "RSK-001", model.RelationCovers),
				},
				"RSK-001": {
					rel("PHS-001", "RSK-001", model.RelationCovers),
				},
			},
			wantBlocked:       true,
			wantBlockingCount: 1,
			wantWarningCount:  0,
		},
		{
			name: "phase with unverified assumption is blocked (medium severity)",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
				"ASM-001": archEntity("ASM-001", model.EntityTypeAssumption, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {
					rel("PHS-001", "ASM-001", model.RelationCovers),
				},
				"ASM-001": {
					rel("PHS-001", "ASM-001", model.RelationCovers),
				},
			},
			wantBlocked:       true,
			wantBlockingCount: 1,
			wantWarningCount:  0,
		},
		{
			name: "phase with low severity issues not blocked",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
				"REQ-001": archEntity("REQ-001", model.EntityTypeRequirement, model.EntityStatusResolved),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {
					rel("PHS-001", "REQ-001", model.RelationCovers),
				},
				"REQ-001": {
					rel("PHS-001", "REQ-001", model.RelationCovers),
				},
			},
			wantBlocked:       false,
			wantBlockingCount: 0,
			wantWarningCount:  0,
		},
		{
			name: "plan with uncovered requirement is blocked",
			target: Target{
				EntityID:   "PLN-001",
				EntityType: model.EntityTypePlan,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PLN-001": execEntity("PLN-001", model.EntityTypePlan, model.EntityStatusActive),
				"REQ-001": archEntity("REQ-001", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PLN-001": {},
				"REQ-001": {},
			},
			wantBlocked:       true,
			wantBlockingCount: 1,
			wantWarningCount:  0,
		},
		{
			name: "phase with no covered entities not blocked",
			target: Target{
				EntityID:   "PHS-001",
				EntityType: model.EntityTypePhase,
				FromStatus: model.EntityStatusActive,
				ToStatus:   model.EntityStatusResolved,
			},
			entities: map[string]model.Entity{
				"PHS-001": execEntity("PHS-001", model.EntityTypePhase, model.EntityStatusActive),
			},
			relations: map[string][]model.Relation{
				"PHS-001": {},
			},
			wantBlocked:       false,
			wantBlockingCount: 0,
			wantWarningCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rf := &mockRelationFetcher{relations: tt.relations}
			ef := &mockEntityFetcher{entities: tt.entities}

			report, err := Enforce(tt.target, rf, ef)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if report == nil {
				t.Fatal("expected non-nil report, got nil")
			}

			if report.Blocked != tt.wantBlocked {
				t.Errorf("Blocked: got %v, want %v", report.Blocked, tt.wantBlocked)
			}

			if len(report.BlockingIssues) != tt.wantBlockingCount {
				t.Errorf("BlockingIssues count: got %d, want %d", len(report.BlockingIssues), tt.wantBlockingCount)
				for _, issue := range report.BlockingIssues {
					t.Logf("  blocking: check=%s severity=%s entity=%s msg=%s", issue.Check, issue.Severity, issue.Entity, issue.Message)
				}
			}

			if len(report.Warnings) != tt.wantWarningCount {
				t.Errorf("Warnings count: got %d, want %d", len(report.Warnings), tt.wantWarningCount)
				for _, issue := range report.Warnings {
					t.Logf("  warning: check=%s severity=%s entity=%s msg=%s", issue.Check, issue.Severity, issue.Entity, issue.Message)
				}
			}

			if report.EntityID != tt.target.EntityID {
				t.Errorf("EntityID: got %q, want %q", report.EntityID, tt.target.EntityID)
			}
			if report.EntityType != tt.target.EntityType {
				t.Errorf("EntityType: got %q, want %q", report.EntityType, tt.target.EntityType)
			}
			if report.FromStatus != tt.target.FromStatus {
				t.Errorf("FromStatus: got %q, want %q", report.FromStatus, tt.target.FromStatus)
			}
			if report.ToStatus != tt.target.ToStatus {
				t.Errorf("ToStatus: got %q, want %q", report.ToStatus, tt.target.ToStatus)
			}
		})
	}
}

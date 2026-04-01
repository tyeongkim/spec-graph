package validate

import (
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestCheckPlanCoverage(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "requirement covered by phase via covers — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
				rel(2, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 0,
		},
		{
			name: "requirement not covered by any phase",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 1,
		},
		{
			name: "no active plan — no issues",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusDraft, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkPlanCoverage(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "plan_coverage" {
					t.Errorf("got check %q; want %q", iss.Check, "plan_coverage")
				}
				if iss.Layer != model.LayerMapping {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerMapping)
				}
			}
		})
	}
}

func TestCheckDeliveryCompleteness(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "completed phase with all covered entities delivered — no issue",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatus("completed"), nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantIssues: 0,
		},
		{
			name: "completed phase covers entity but does not deliver",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatus("completed"), nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
		{
			name: "active phase — not checked",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 0,
		},
		{
			name: "delivered via delivers — no issue",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatus("completed"), nil),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "API-1", model.RelationCovers),
				rel(2, "PHS-1", "API-1", model.RelationDelivers),
			},
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkDeliveryCompleteness(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "delivery_completeness" {
					t.Errorf("got check %q; want %q", iss.Check, "delivery_completeness")
				}
			}
		})
	}
}

func TestCheckMappingConsistency(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "mapping to active entity — no issue",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 0,
		},
		{
			name: "mapping to deprecated entity",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusDeprecated),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
		{
			name: "mapping to superseded entity",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-2", "REQ-1", model.RelationSupersedes),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkMappingConsistency(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "mapping_consistency" {
					t.Errorf("got check %q; want %q", iss.Check, "mapping_consistency")
				}
			}
		})
	}
}

func TestCheckInvalidMappingEdges(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "valid mapping edge — phase covers requirement",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 0,
		},
		{
			name: "invalid mapping edge — requirement covers phase",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "PHS-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkInvalidMappingEdges(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "invalid_mapping_edges" {
					t.Errorf("got check %q; want %q", iss.Check, "invalid_mapping_edges")
				}
			}
		})
	}
}

func TestCheckGates(t *testing.T) {
	phs1 := "PHS-1"

	tests := []struct {
		name       string
		phase      *string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
		wantChecks []string
	}{
		{
			name:  "unresolved question in phase scope",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "QST-1", model.RelationCovers),
			},
			wantIssues: 1,
			wantChecks: []string{"gates"},
		},
		{
			name:  "resolved question — no issue",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "QST-1", model.RelationCovers),
				rel(2, "DEC-1", "QST-1", model.RelationAnswers),
			},
			wantIssues: 0,
		},
		{
			name:  "unmitigated risk in phase scope",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("RSK-1", model.EntityTypeRisk, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "RSK-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
		{
			name:  "mitigated risk — no issue",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("RSK-1", model.EntityTypeRisk, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "RSK-1", model.RelationCovers),
				rel(2, "DEC-1", "RSK-1", model.RelationMitigates),
			},
			wantIssues: 0,
		},
		{
			name:  "unverified assumption in phase scope",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("ASM-1", model.EntityTypeAssumption, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "ASM-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
		{
			name:  "requirement depends on draft decision",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusDraft),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "DEC-1", model.RelationDependsOn),
			},
			wantIssues: 1,
		},
		{
			name:  "requirement depends on active decision — no issue",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "DEC-1", model.RelationDependsOn),
			},
			wantIssues: 0,
		},
		{
			name:  "no phase specified — checks all active phases",
			phase: nil,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "QST-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
		{
			name:  "entity not covered by phase — no issue",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			opts := ValidateOptions{Phase: tt.phase}
			issues := checkGates(opts, rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "gates" {
					t.Errorf("got check %q; want %q", iss.Check, "gates")
				}
				if iss.Layer != model.LayerMapping {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerMapping)
				}
			}
		})
	}
}

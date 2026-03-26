package validate

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestCheckPhaseOrder(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "unique orders — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":1}`)),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":2}`)),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
				rel(2, "PHS-2", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 0,
		},
		{
			name: "duplicate order in same plan",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":1}`)),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":1}`)),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
				rel(2, "PHS-2", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 2,
		},
		{
			name: "same order in different plans — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PLN-2", model.EntityTypePlan, model.EntityStatusDraft, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":1}`)),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, json.RawMessage(`{"order":1}`)),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
				rel(2, "PHS-2", "PLN-2", model.RelationBelongsTo),
			},
			wantIssues: 0,
		},
		{
			name: "phase without order metadata — skipped",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkPhaseOrder(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "phase_order" {
					t.Errorf("got check %q; want %q", iss.Check, "phase_order")
				}
				if iss.Layer != model.LayerExec {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerExec)
				}
			}
		})
	}
}

func TestCheckSingleActivePlan(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		wantIssues int
	}{
		{
			name: "one active plan — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
			},
			wantIssues: 0,
		},
		{
			name: "no active plans — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusDraft, nil),
			},
			wantIssues: 0,
		},
		{
			name: "multiple active plans",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PLN-2", model.EntityTypePlan, model.EntityStatusActive, nil),
			},
			wantIssues: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			issues := checkSingleActivePlan(ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "single_active_plan" {
					t.Errorf("got check %q; want %q", iss.Check, "single_active_plan")
				}
			}
		})
	}
}

func TestCheckOrphanPhases(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "phase belongs to plan — no issue",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 0,
		},
		{
			name: "orphan phase — no belongs_to",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations:  nil,
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkOrphanPhases(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "orphan_phases" {
					t.Errorf("got check %q; want %q", iss.Check, "orphan_phases")
				}
			}
		})
	}
}

func TestCheckExecCycles(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantCyclic bool
	}{
		{
			name: "no cycle in blocks",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PHS-2", model.RelationBlocks),
			},
			wantCyclic: false,
		},
		{
			name: "cycle in blocks A→B→A",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PHS-2", model.RelationBlocks),
				rel(2, "PHS-2", "PHS-1", model.RelationBlocks),
			},
			wantCyclic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkExecCycles(rf, ef)
			hasCycle := len(issues) > 0
			if hasCycle != tt.wantCyclic {
				t.Errorf("got cycle=%v; want %v; issues=%+v", hasCycle, tt.wantCyclic, issues)
			}
			for _, iss := range issues {
				if iss.Check != "exec_cycles" {
					t.Errorf("got check %q; want %q", iss.Check, "exec_cycles")
				}
			}
		})
	}
}

func TestCheckInvalidExecEdges(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "valid exec edge — phase belongs_to plan",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "PLN-1", model.RelationBelongsTo),
			},
			wantIssues: 0,
		},
		{
			name: "invalid exec edge — plan belongs_to phase",
			entities: []model.Entity{
				execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil),
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "PLN-1", "PHS-1", model.RelationBelongsTo),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkInvalidExecEdges(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "invalid_exec_edges" {
					t.Errorf("got check %q; want %q", iss.Check, "invalid_exec_edges")
				}
			}
		})
	}
}

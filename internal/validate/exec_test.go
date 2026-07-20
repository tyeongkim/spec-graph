package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestTaskGraphRejectsCrossPhaseAndCycles(t *testing.T) {
	tasks := []model.Entity{
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
		execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
		execEntity("TSK-1", model.EntityTypeTask, model.EntityStatusActive, nil),
		execEntity("TSK-2", model.EntityTypeTask, model.EntityStatusActive, nil),
		execEntity("TSK-3", model.EntityTypeTask, model.EntityStatusDeprecated, nil),
		execEntity("TSK-4", model.EntityTypeTask, model.EntityStatusActive, nil),
	}
	tests := []struct {
		name      string
		relations []model.Relation
		wantIDs   []string
		wantText  string
	}{
		{
			name: "zero parent",
			relations: []model.Relation{
				rel(1, "TSK-2", "PHS-1", model.RelationBelongsTo),
				rel(2, "TSK-3", "PHS-1", model.RelationBelongsTo),
				rel(3, "TSK-4", "PHS-1", model.RelationBelongsTo),
			},
			wantIDs: []string{"TSK-1"}, wantText: "zero parent",
		},
		{
			name: "multiple parents",
			relations: []model.Relation{
				rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "TSK-1", "PHS-2", model.RelationBelongsTo),
				rel(3, "TSK-2", "PHS-1", model.RelationBelongsTo), rel(4, "TSK-3", "PHS-1", model.RelationBelongsTo), rel(5, "TSK-4", "PHS-1", model.RelationBelongsTo),
			},
			wantIDs: []string{"TSK-1", "PHS-1", "PHS-2"}, wantText: "multiple parent",
		},
		{
			name: "cross phase",
			relations: []model.Relation{
				rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "TSK-2", "PHS-2", model.RelationBelongsTo),
				rel(3, "TSK-3", "PHS-1", model.RelationBelongsTo), rel(4, "TSK-4", "PHS-1", model.RelationBelongsTo),
				rel(5, "TSK-1", "TSK-2", model.RelationTaskDependsOn),
			},
			wantIDs: []string{"TSK-1", "TSK-2", "PHS-1", "PHS-2"}, wantText: "cross-phase",
		},
		{
			name: "self dependency",
			relations: []model.Relation{
				rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "TSK-2", "PHS-1", model.RelationBelongsTo),
				rel(3, "TSK-3", "PHS-1", model.RelationBelongsTo), rel(4, "TSK-4", "PHS-1", model.RelationBelongsTo),
				rel(5, "TSK-1", "TSK-1", model.RelationTaskDependsOn),
			},
			wantIDs: []string{"TSK-1"}, wantText: "self-dependency",
		},
		{
			name: "deprecated prerequisite",
			relations: []model.Relation{
				rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "TSK-2", "PHS-1", model.RelationBelongsTo),
				rel(3, "TSK-3", "PHS-1", model.RelationBelongsTo), rel(4, "TSK-4", "PHS-1", model.RelationBelongsTo),
				rel(5, "TSK-1", "TSK-3", model.RelationTaskDependsOn),
			},
			wantIDs: []string{"TSK-1", "TSK-3"}, wantText: "deprecated task",
		},
		{
			name: "cycle",
			relations: []model.Relation{
				rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "TSK-2", "PHS-1", model.RelationBelongsTo),
				rel(3, "TSK-3", "PHS-1", model.RelationBelongsTo), rel(4, "TSK-4", "PHS-1", model.RelationBelongsTo),
				rel(5, "TSK-1", "TSK-2", model.RelationTaskDependsOn), rel(6, "TSK-2", "TSK-4", model.RelationTaskDependsOn),
				rel(7, "TSK-4", "TSK-1", model.RelationTaskDependsOn),
			},
			wantIDs: []string{"TSK-1", "TSK-2", "TSK-4"}, wantText: "cycle members",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := checkTaskGraph(newMockRelationFetcher(test.relations...), newMockEntityFetcher(tasks...))
			var matching strings.Builder
			for _, issue := range issues {
				if strings.Contains(issue.Message, test.wantText) {
					matching.WriteString(issue.Entity)
					matching.WriteString(" ")
					matching.WriteString(issue.Message)
				}
			}
			text := matching.String()
			if text == "" {
				t.Fatalf("no task_graph issue containing %q: %+v", test.wantText, issues)
			}
			for _, id := range test.wantIDs {
				if !strings.Contains(text, id) {
					t.Errorf("issue %q does not name offending member %s", text, id)
				}
			}
		})
	}
}

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

func TestCheckOrphanChanges(t *testing.T) {
	tests := []struct {
		name         string
		entities     []model.Entity
		relations    []model.Relation
		wantIssues   int
		wantSeverity Severity
	}{
		{
			name: "CHG with covers relation — no issue",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "CHG-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 0,
		},
		{
			name: "CHG with no relation, status=draft — medium severity",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusDraft, nil),
			},
			relations:    nil,
			wantIssues:   1,
			wantSeverity: SeverityMedium,
		},
		{
			name: "CHG with no relation, status=active — high severity",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusActive, nil),
			},
			relations:    nil,
			wantIssues:   1,
			wantSeverity: SeverityHigh,
		},
		{
			name: "CHG with no relation, status=resolved — high severity",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusResolved, nil),
			},
			relations:    nil,
			wantIssues:   1,
			wantSeverity: SeverityHigh,
		},
		{
			name: "CHG with no relation, status=deprecated — high severity",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusDeprecated, nil),
			},
			relations:    nil,
			wantIssues:   1,
			wantSeverity: SeverityHigh,
		},
		{
			name: "CHG only related to another CHG — still orphan",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusActive, nil),
				execEntity("CHG-2", model.EntityTypeChange, model.EntityStatusActive, nil),
			},
			relations: []model.Relation{
				rel(1, "CHG-1", "CHG-2", model.RelationReferences),
			},
			wantIssues:   2,
			wantSeverity: SeverityHigh,
		},
		{
			name: "CHG as relation target — no issue",
			entities: []model.Entity{
				execEntity("CHG-1", model.EntityTypeChange, model.EntityStatusActive, nil),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "CHG-1", model.RelationReferences),
			},
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkOrphanChanges(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "orphan_changes" {
					t.Errorf("got check %q; want %q", iss.Check, "orphan_changes")
				}
				if iss.Layer != model.LayerExec {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerExec)
				}
				if tt.wantSeverity != "" && iss.Severity != tt.wantSeverity {
					t.Errorf("got severity %q; want %q", iss.Severity, tt.wantSeverity)
				}
			}
		})
	}
}

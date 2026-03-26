package validate

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

type mockEntityFetcher struct {
	entities map[string]model.Entity
}

func newMockEntityFetcher(entities ...model.Entity) *mockEntityFetcher {
	m := &mockEntityFetcher{entities: make(map[string]model.Entity, len(entities))}
	for _, e := range entities {
		m.entities[e.ID] = e
	}
	return m
}

func (m *mockEntityFetcher) Get(id string) (model.Entity, error) {
	e, ok := m.entities[id]
	if !ok {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return e, nil
}

func (m *mockEntityFetcher) List(filters EntityListFilters) ([]model.Entity, error) {
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

type mockRelationFetcher struct {
	relations map[string][]model.Relation
}

func newMockRelationFetcher(rels ...model.Relation) *mockRelationFetcher {
	m := &mockRelationFetcher{relations: make(map[string][]model.Relation)}
	for _, r := range rels {
		m.relations[r.FromID] = append(m.relations[r.FromID], r)
		if r.FromID != r.ToID {
			m.relations[r.ToID] = append(m.relations[r.ToID], r)
		}
	}
	return m
}

func (m *mockRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	return m.relations[entityID], nil
}

func archEntity(id string, typ model.EntityType, status model.EntityStatus) model.Entity {
	return model.Entity{
		ID:       id,
		Type:     typ,
		Layer:    model.LayerArch,
		Status:   status,
		Metadata: json.RawMessage(`{}`),
	}
}

func execEntity(id string, typ model.EntityType, status model.EntityStatus, meta json.RawMessage) model.Entity {
	if meta == nil {
		meta = json.RawMessage(`{}`)
	}
	return model.Entity{
		ID:       id,
		Type:     typ,
		Layer:    model.LayerExec,
		Status:   status,
		Metadata: meta,
	}
}

func rel(id int, from, to string, relType model.RelationType) model.Relation {
	return model.Relation{
		ID:     id,
		FromID: from,
		ToID:   to,
		Type:   relType,
		Layer:  model.LayerForRelationType(relType),
	}
}

func TestCheckOrphans(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "no orphans — all entities have arch relations",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "REQ-1", model.RelationImplements),
			},
			wantIssues: 0,
		},
		{
			name: "detects orphan — entity with no relations",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 1,
		},
		{
			name: "skips deprecated entities",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusDeprecated),
			},
			relations:  nil,
			wantIssues: 0,
		},
		{
			name: "entity with only mapping relation is still orphan",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkOrphans(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "orphans" {
					t.Errorf("got check %q; want %q", iss.Check, "orphans")
				}
				if iss.Layer != model.LayerArch {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerArch)
				}
			}
		})
	}
}

func TestCheckCoverage(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "fully covered requirement — has implements and has_criterion",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
				archEntity("ACT-1", model.EntityTypeCriterion, model.EntityStatusActive),
				archEntity("TST-1", model.EntityTypeTest, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "REQ-1", model.RelationImplements),
				rel(2, "REQ-1", "ACT-1", model.RelationHasCriterion),
				rel(3, "TST-1", "ACT-1", model.RelationVerifies),
			},
			wantIssues: 0,
		},
		{
			name: "requirement missing implements — 1 issue",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("ACT-1", model.EntityTypeCriterion, model.EntityStatusActive),
				archEntity("TST-1", model.EntityTypeTest, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(2, "REQ-1", "ACT-1", model.RelationHasCriterion),
				rel(3, "TST-1", "ACT-1", model.RelationVerifies),
			},
			wantIssues: 1,
		},
		{
			name: "requirement missing criterion — 1 issue",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "REQ-1", model.RelationImplements),
			},
			wantIssues: 1,
		},
		{
			name: "criterion without verification",
			entities: []model.Entity{
				archEntity("ACT-1", model.EntityTypeCriterion, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 1,
		},
		{
			name: "interface triggers state but no verifying test",
			entities: []model.Entity{
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
				archEntity("STT-1", model.EntityTypeState, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "STT-1", model.RelationTriggers),
			},
			wantIssues: 1,
		},
		{
			name: "interface triggers state and has verifying test — no issue",
			entities: []model.Entity{
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
				archEntity("STT-1", model.EntityTypeState, model.EntityStatusActive),
				archEntity("TST-1", model.EntityTypeTest, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "STT-1", model.RelationTriggers),
				rel(2, "TST-1", "API-1", model.RelationVerifies),
			},
			wantIssues: 0,
		},
		{
			name: "draft requirement skipped",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusDraft),
			},
			relations:  nil,
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkCoverage(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "coverage" {
					t.Errorf("got check %q; want %q", iss.Check, "coverage")
				}
			}
		})
	}
}

func TestCheckCycles(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantCyclic bool
	}{
		{
			name: "no cycle",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "REQ-2", model.RelationDependsOn),
			},
			wantCyclic: false,
		},
		{
			name: "detects cycle A→B→A",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "REQ-2", model.RelationDependsOn),
				rel(2, "REQ-2", "REQ-1", model.RelationDependsOn),
			},
			wantCyclic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkCycles(rf, ef)
			hasCycle := len(issues) > 0
			if hasCycle != tt.wantCyclic {
				t.Errorf("got cycle=%v; want %v; issues=%+v", hasCycle, tt.wantCyclic, issues)
			}
			for _, iss := range issues {
				if iss.Check != "cycles" {
					t.Errorf("got check %q; want %q", iss.Check, "cycles")
				}
				if iss.Severity != SeverityHigh {
					t.Errorf("got severity %q; want %q", iss.Severity, SeverityHigh)
				}
			}
		})
	}
}

func TestCheckConflicts(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "no conflicts",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "REQ-2", model.RelationDependsOn),
			},
			wantIssues: 0,
		},
		{
			name: "detects active conflict",
			entities: []model.Entity{
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
				archEntity("DEC-2", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "DEC-2", model.RelationConflictsWith),
			},
			wantIssues: 1,
		},
		{
			name: "conflict with deprecated entity — no issue",
			entities: []model.Entity{
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
				archEntity("DEC-2", model.EntityTypeDecision, model.EntityStatusDeprecated),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "DEC-2", model.RelationConflictsWith),
			},
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkConflicts(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "conflicts" {
					t.Errorf("got check %q; want %q", iss.Check, "conflicts")
				}
			}
		})
	}
}

func TestCheckInvalidEdges(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "valid edge — no issue",
			entities: []model.Entity{
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "API-1", "REQ-1", model.RelationImplements),
			},
			wantIssues: 0,
		},
		{
			name: "invalid edge — requirement cannot implement requirement",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "REQ-2", model.RelationImplements),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkInvalidEdges(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "invalid_edges" {
					t.Errorf("got check %q; want %q", iss.Check, "invalid_edges")
				}
			}
		})
	}
}

func TestCheckSupersededRefs(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name: "no superseded refs — clean",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-1", "REQ-2", model.RelationDependsOn),
			},
			wantIssues: 0,
		},
		{
			name: "active entity references superseded entity",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusDeprecated),
				archEntity("REQ-3", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "REQ-3", "REQ-2", model.RelationSupersedes),
				rel(2, "REQ-1", "REQ-2", model.RelationDependsOn),
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkSupersededRefs(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "superseded_refs" {
					t.Errorf("got check %q; want %q", iss.Check, "superseded_refs")
				}
			}
		})
	}
}

func TestCheckUnresolved(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantIssues int
	}{
		{
			name:       "no unresolved entities — clean",
			entities:   nil,
			relations:  nil,
			wantIssues: 0,
		},
		{
			name: "active question without answer",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 1,
		},
		{
			name: "active question with answer — no issue",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "QST-1", model.RelationAnswers),
			},
			wantIssues: 0,
		},
		{
			name: "active assumption always flagged",
			entities: []model.Entity{
				archEntity("ASM-1", model.EntityTypeAssumption, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 1,
		},
		{
			name: "active risk without mitigation",
			entities: []model.Entity{
				archEntity("RSK-1", model.EntityTypeRisk, model.EntityStatusActive),
			},
			relations:  nil,
			wantIssues: 1,
		},
		{
			name: "active risk with mitigation — no issue",
			entities: []model.Entity{
				archEntity("RSK-1", model.EntityTypeRisk, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "RSK-1", model.RelationMitigates),
			},
			wantIssues: 0,
		},
		{
			name: "resolved question — no issue",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusResolved),
			},
			relations:  nil,
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			issues := checkUnresolved(rf, ef)
			if len(issues) != tt.wantIssues {
				t.Errorf("got %d issues; want %d; issues=%+v", len(issues), tt.wantIssues, issues)
			}
			for _, iss := range issues {
				if iss.Check != "unresolved" {
					t.Errorf("got check %q; want %q", iss.Check, "unresolved")
				}
			}
		})
	}
}

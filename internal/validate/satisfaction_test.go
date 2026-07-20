package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestSatisfactionResolvedStatuses(t *testing.T) {
	tests := []struct {
		name       string
		entity     model.Entity
		wantStatus SatisfactionItemStatus
	}{
		{
			name:       "resolved assumption",
			entity:     archEntity("ASM-1", model.EntityTypeAssumption, model.EntityStatusResolved),
			wantStatus: SatisfactionSatisfied,
		},
		{
			name:       "resolved test",
			entity:     archEntity("TST-1", model.EntityTypeTest, model.EntityStatusResolved),
			wantStatus: SatisfactionSatisfied,
		},
		{
			name:       "resolved decision",
			entity:     archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusResolved),
			wantStatus: SatisfactionSatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entity)
			rf := newMockRelationFetcher()
			member := closureMember{
				entityID: tt.entity.ID,
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     "PHS-1",
			}

			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)

			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestComputePhaseClosure(t *testing.T) {
	tests := []struct {
		name              string
		phaseID           string
		includeReferences bool
		entities          []model.Entity
		relations         []model.Relation
		wantMandatory     []string
		wantAdvisory      []string
	}{
		{
			name:    "directly covered entities only",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-2", model.RelationCovers),
			},
			wantMandatory: []string{"REQ-1", "REQ-2"},
		},
		{
			name:    "1-depth depends_on outbound from covered entity",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "DEC-1", model.RelationDependsOn),
			},
			wantMandatory: []string{"REQ-1", "DEC-1"},
		},
		{
			name:    "1-depth implements inbound to covered entity (interface implements REQ)",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "API-1", "REQ-1", model.RelationImplements),
			},
			wantMandatory: []string{"REQ-1", "API-1"},
		},
		{
			name:              "references excluded by default",
			phaseID:           "PHS-1",
			includeReferences: false,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "QST-1", model.RelationReferences),
			},
			wantMandatory: []string{"REQ-1"},
		},
		{
			name:              "references included as advisory when opt-in",
			phaseID:           "PHS-1",
			includeReferences: true,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "QST-1", model.RelationReferences),
			},
			wantMandatory: []string{"REQ-1"},
			wantAdvisory:  []string{"QST-1"},
		},
		{
			name:              "mandatory wins when discovered after advisory via different covered entities",
			phaseID:           "PHS-1",
			includeReferences: true,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-2", model.RelationCovers),
				rel(3, "REQ-1", "DEC-1", model.RelationReferences),
				rel(4, "REQ-2", "DEC-1", model.RelationDependsOn),
			},
			wantMandatory: []string{"REQ-1", "REQ-2", "DEC-1"},
			wantAdvisory:  nil,
		},
		{
			name:              "mandatory wins when same target via depends_on and references from same covered",
			phaseID:           "PHS-1",
			includeReferences: true,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "DEC-1", model.RelationDependsOn),
				rel(3, "REQ-1", "DEC-1", model.RelationReferences),
			},
			wantMandatory: []string{"REQ-1", "DEC-1"},
			wantAdvisory:  nil,
		},
		{
			name:              "directly covered overrides advisory references from another covered",
			phaseID:           "PHS-1",
			includeReferences: true,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-2", model.RelationCovers),
				rel(3, "REQ-1", "REQ-2", model.RelationReferences),
			},
			wantMandatory: []string{"REQ-1", "REQ-2"},
			wantAdvisory:  nil,
		},
		{
			name:    "depth limited to 1 — second-hop not included",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
				archEntity("DEC-2", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-1", "DEC-1", model.RelationDependsOn),
				rel(3, "DEC-1", "DEC-2", model.RelationDependsOn),
			},
			wantMandatory: []string{"REQ-1", "DEC-1"},
		},
		{
			name:    "inbound depends_on does not pull source into closure",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "REQ-2", "REQ-1", model.RelationDependsOn),
			},
			wantMandatory: []string{"REQ-1"},
		},
		{
			name:    "outbound implements does not pull target into closure",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("API-1", model.EntityTypeInterface, model.EntityStatusActive),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "API-1", model.RelationCovers),
				rel(2, "API-1", "REQ-1", model.RelationImplements),
			},
			wantMandatory: []string{"API-1"},
		},
		{
			name:    "phase with no covers — empty closure",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
			},
			relations:     nil,
			wantMandatory: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rf := newMockRelationFetcher(tt.relations...)
			closure, err := computePhaseClosure(tt.phaseID, tt.includeReferences, rf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotMandatory := []string{}
			gotAdvisory := []string{}
			for _, m := range closure {
				if m.class == closureMandatory {
					gotMandatory = append(gotMandatory, m.entityID)
				} else {
					gotAdvisory = append(gotAdvisory, m.entityID)
				}
			}

			if !sameStringSet(gotMandatory, tt.wantMandatory) {
				t.Errorf("mandatory: got %v; want %v", gotMandatory, tt.wantMandatory)
			}
			if !sameStringSet(gotAdvisory, tt.wantAdvisory) {
				t.Errorf("advisory: got %v; want %v", gotAdvisory, tt.wantAdvisory)
			}
		})
	}
}

func TestEvaluateMember_RequirementWithDelivers(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantStatus SatisfactionItemStatus
	}{
		{
			name: "phase delivers requirement, phase active — satisfied",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name: "no inbound delivers — unsatisfied",
			entities: []model.Entity{
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations:  nil,
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name: "phase delivers requirement, phase draft — unsatisfied (Layer 3 fail)",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusDraft, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name: "phase delivers requirement, phase resolved — satisfied",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusResolved, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionSatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			member := closureMember{
				entityID: "REQ-1",
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     "PHS-1",
			}
			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)
			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestEvaluateMember_QuestionWithAnswers(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantStatus SatisfactionItemStatus
	}{
		{
			name: "decision answers question, decision active — satisfied",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "QST-1", model.RelationAnswers),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name: "no inbound answers — unsatisfied",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations:  nil,
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name: "decision answers but draft — unsatisfied",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusDraft),
			},
			relations: []model.Relation{
				rel(1, "DEC-1", "QST-1", model.RelationAnswers),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			member := closureMember{
				entityID: "QST-1",
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     "PHS-1",
			}
			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)
			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestEvaluateMember_StatusOnlyRules(t *testing.T) {
	tests := []struct {
		name       string
		memberID   string
		entities   []model.Entity
		wantStatus SatisfactionItemStatus
	}{
		{
			name:     "resolved assumption — satisfied",
			memberID: "ASM-1",
			entities: []model.Entity{
				archEntity("ASM-1", model.EntityTypeAssumption, model.EntityStatusResolved),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name:     "active assumption — unsatisfied",
			memberID: "ASM-1",
			entities: []model.Entity{
				archEntity("ASM-1", model.EntityTypeAssumption, model.EntityStatusActive),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name:     "active decision — satisfied",
			memberID: "DEC-1",
			entities: []model.Entity{
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusActive),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name:     "draft decision — unsatisfied",
			memberID: "DEC-1",
			entities: []model.Entity{
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusDraft),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name:     "resolved decision — satisfied",
			memberID: "DEC-1",
			entities: []model.Entity{
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusResolved),
			},
			wantStatus: SatisfactionSatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher()
			member := closureMember{
				entityID: tt.memberID,
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     "PHS-1",
			}
			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)
			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestEvaluateMember_AdvisoryNeverBlocks(t *testing.T) {
	tests := []struct {
		name     string
		memberID string
		entities []model.Entity
	}{
		{
			name:     "advisory entity exists",
			memberID: "QST-1",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
			},
		},
		{
			name:     "advisory entity missing — must still be advisory, not unsatisfied",
			memberID: "QST-MISSING",
			entities: []model.Entity{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher()
			member := closureMember{
				entityID: tt.memberID,
				class:    closureAdvisory,
				origin:   model.RelationReferences,
				from:     "REQ-1",
			}
			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)
			if item.Status != SatisfactionAdvisory {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, SatisfactionAdvisory, item.Reason)
			}
		})
	}
}

func TestEvaluateMember_MultipleEvidenceSourcesExistential(t *testing.T) {
	tests := []struct {
		name       string
		entities   []model.Entity
		relations  []model.Relation
		wantStatus SatisfactionItemStatus
	}{
		{
			name: "first answers source draft, second answers source active — satisfied (existential)",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-DRAFT", model.EntityTypeDecision, model.EntityStatusDraft),
				archEntity("DEC-ACTIVE", model.EntityTypeDecision, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "DEC-DRAFT", "QST-1", model.RelationAnswers),
				rel(2, "DEC-ACTIVE", "QST-1", model.RelationAnswers),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name: "all answers sources fail Layer 3 — unsatisfied",
			entities: []model.Entity{
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("DEC-A", model.EntityTypeDecision, model.EntityStatusDraft),
				archEntity("DEC-B", model.EntityTypeDecision, model.EntityStatusDraft),
			},
			relations: []model.Relation{
				rel(1, "DEC-A", "QST-1", model.RelationAnswers),
				rel(2, "DEC-B", "QST-1", model.RelationAnswers),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			member := closureMember{
				entityID: "QST-1",
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     "PHS-1",
			}
			item := evaluateMember(member, member.from, map[string]bool{member.from: true}, ef, rf)
			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestEvaluateMember_DeliversScopedToCurrentPhase(t *testing.T) {
	tests := []struct {
		name       string
		phaseID    string
		entities   []model.Entity
		relations  []model.Relation
		wantStatus SatisfactionItemStatus
	}{
		{
			name:    "current phase delivers — satisfied",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionSatisfied,
		},
		{
			name:    "only another phase delivers — unsatisfied (cross-phase rejected)",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-2", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionUnsatisfied,
		},
		{
			name:    "current phase + another phase deliver — satisfied via current",
			phaseID: "PHS-1",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationDelivers),
				rel(2, "PHS-2", "REQ-1", model.RelationDelivers),
			},
			wantStatus: SatisfactionSatisfied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			member := closureMember{
				entityID: "REQ-1",
				class:    closureMandatory,
				origin:   model.RelationCovers,
				from:     tt.phaseID,
			}
			item := evaluateMember(member, tt.phaseID, map[string]bool{tt.phaseID: true}, ef, rf)
			if item.Status != tt.wantStatus {
				t.Errorf("got status %q; want %q (reason: %s)", item.Status, tt.wantStatus, item.Reason)
			}
		})
	}
}

func TestSelectPhases_RejectsNonPhaseEntity(t *testing.T) {
	reqID := "REQ-1"
	ef := newMockEntityFetcher(
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	)

	_, err := selectPhases(ValidateOptions{Phase: &reqID}, ef)
	if err == nil {
		t.Fatal("expected error when Phase points to non-phase entity; got nil")
	}
}

func TestCheckPhaseSatisfaction_NonPhaseIDPropagatedAsIssue(t *testing.T) {
	reqID := "REQ-1"
	ef := newMockEntityFetcher(
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	)
	rf := newMockRelationFetcher()

	issues, reports := checkPhaseSatisfaction(ValidateOptions{Phase: &reqID}, rf, ef)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue when phase ID is not a phase; got %d", len(issues))
	}
	if issues[0].Check != "phase_satisfaction" {
		t.Errorf("got check %q; want %q", issues[0].Check, "phase_satisfaction")
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("got severity %q; want %q", issues[0].Severity, SeverityHigh)
	}
	if issues[0].Entity != reqID {
		t.Errorf("got entity %q; want %q", issues[0].Entity, reqID)
	}
	if !strings.Contains(issues[0].Message, "not phase") {
		t.Errorf("expected message to mention type mismatch; got %q", issues[0].Message)
	}
	if len(reports) != 0 {
		t.Errorf("expected no reports when phase selection fails; got %d", len(reports))
	}
}

func TestEvaluateMember_CrossPhaseRejectionDiagnostic(t *testing.T) {
	ef := newMockEntityFetcher(
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
		execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil),
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	)
	rf := newMockRelationFetcher(
		rel(1, "PHS-2", "REQ-1", model.RelationDelivers),
	)
	member := closureMember{
		entityID: "REQ-1",
		class:    closureMandatory,
		origin:   model.RelationCovers,
		from:     "PHS-1",
	}

	item := evaluateMember(member, "PHS-1", map[string]bool{"PHS-1": true}, ef, rf)
	if item.Status != SatisfactionUnsatisfied {
		t.Fatalf("got status %q; want %q", item.Status, SatisfactionUnsatisfied)
	}
	if !strings.Contains(item.Reason, "PHS-1") {
		t.Errorf("reason should name the validating phase PHS-1; got %q", item.Reason)
	}
	if !strings.Contains(item.Reason, "PHS-2") {
		t.Errorf("reason should name the cross-phase deliverer PHS-2 for diagnostics; got %q", item.Reason)
	}
}

func TestComputePhaseSatisfaction_RatioAndDetails(t *testing.T) {
	phs1 := "PHS-1"
	tests := []struct {
		name              string
		includeReferences bool
		entities          []model.Entity
		relations         []model.Relation
		wantSatisfied     int
		wantTotal         int
		wantAdvisory      int
	}{
		{
			name: "all satisfied — full ratio",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantSatisfied: 1,
			wantTotal:     1,
		},
		{
			name: "partial satisfied — partial ratio",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-2", model.RelationCovers),
				rel(3, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantSatisfied: 1,
			wantTotal:     2,
		},
		{
			name:              "advisory does not affect ratio",
			includeReferences: true,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
				archEntity("QST-2", model.EntityTypeQuestion, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
				rel(3, "REQ-1", "QST-1", model.RelationReferences),
				rel(4, "REQ-1", "QST-2", model.RelationReferences),
			},
			wantSatisfied: 1,
			wantTotal:     1,
			wantAdvisory:  2,
		},
		{
			name: "transitive depends_on counted in total — draft decision unsatisfied",
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusDraft),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
				rel(3, "REQ-1", "DEC-1", model.RelationDependsOn),
			},
			wantSatisfied: 1,
			wantTotal:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			report, err := computePhaseSatisfaction(phs1, tt.includeReferences, rf, ef)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Satisfied != tt.wantSatisfied {
				t.Errorf("satisfied: got %d; want %d", report.Satisfied, tt.wantSatisfied)
			}
			if report.Total != tt.wantTotal {
				t.Errorf("total: got %d; want %d", report.Total, tt.wantTotal)
			}
			if report.AdvisoryCount != tt.wantAdvisory {
				t.Errorf("advisory: got %d; want %d", report.AdvisoryCount, tt.wantAdvisory)
			}
		})
	}
}

func TestCheckPhaseSatisfaction_IssuesForUnsatisfied(t *testing.T) {
	phs1 := "PHS-1"
	tests := []struct {
		name           string
		phase          *string
		entities       []model.Entity
		relations      []model.Relation
		wantIssueCount int
	}{
		{
			name:  "no issues when all satisfied",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
			},
			wantIssueCount: 0,
		},
		{
			name:  "one issue per unsatisfied mandatory member",
			phase: &phs1,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
				archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
				rel(2, "PHS-1", "REQ-2", model.RelationCovers),
			},
			wantIssueCount: 2,
		},
		{
			name:  "no phase specified — checks all active phases",
			phase: nil,
			entities: []model.Entity{
				execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
				archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
			},
			relations: []model.Relation{
				rel(1, "PHS-1", "REQ-1", model.RelationCovers),
			},
			wantIssueCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := newMockEntityFetcher(tt.entities...)
			rf := newMockRelationFetcher(tt.relations...)
			opts := ValidateOptions{Phase: tt.phase}
			issues, reports := checkPhaseSatisfaction(opts, rf, ef)
			if len(issues) != tt.wantIssueCount {
				t.Errorf("got %d issues; want %d (issues=%+v)", len(issues), tt.wantIssueCount, issues)
			}
			for _, iss := range issues {
				if iss.Check != "phase_satisfaction" {
					t.Errorf("got check %q; want %q", iss.Check, "phase_satisfaction")
				}
				if iss.Layer != model.LayerMapping {
					t.Errorf("got layer %q; want %q", iss.Layer, model.LayerMapping)
				}
				if iss.Severity != SeverityHigh {
					t.Errorf("got severity %q; want %q", iss.Severity, SeverityHigh)
				}
			}
			if len(reports) == 0 {
				t.Error("expected at least one phase satisfaction report")
			}
		})
	}
}

func TestValidate_PhaseSatisfaction_TransitiveScopeFiltering(t *testing.T) {
	phs1 := "PHS-1"
	mappingLayer := model.LayerMapping
	entities := []model.Entity{
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
		archEntity("DEC-1", model.EntityTypeDecision, model.EntityStatusDraft),
	}
	relations := []model.Relation{
		rel(1, "PHS-1", "REQ-1", model.RelationCovers),
		rel(2, "PHS-1", "REQ-1", model.RelationDelivers),
		rel(3, "REQ-1", "DEC-1", model.RelationDependsOn),
	}

	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher(relations...)

	result, err := Validate(ValidateOptions{
		Layer:  &mappingLayer,
		Checks: []string{"phase_satisfaction"},
		Phase:  &phs1,
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDecIssue := false
	for _, iss := range result.Issues {
		if iss.Check == "phase_satisfaction" && iss.Entity == "DEC-1" {
			foundDecIssue = true
		}
	}
	if !foundDecIssue {
		t.Errorf("transitive mandatory member DEC-1 issue was filtered out by phase scope; issues=%+v", result.Issues)
	}
	if result.Valid {
		t.Errorf("result should not be valid; got Valid=true with issues=%+v", result.Issues)
	}
}

func TestValidate_PhaseSatisfactionAttachedToResult(t *testing.T) {
	phs1 := "PHS-1"
	mappingLayer := model.LayerMapping
	entities := []model.Entity{
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	}
	relations := []model.Relation{
		rel(1, "PHS-1", "REQ-1", model.RelationCovers),
	}

	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher(relations...)

	result, err := Validate(ValidateOptions{
		Layer:  &mappingLayer,
		Checks: []string{"phase_satisfaction"},
		Phase:  &phs1,
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Satisfaction) != 1 {
		t.Fatalf("got %d satisfaction reports; want 1", len(result.Satisfaction))
	}
	report := result.Satisfaction[0]
	if report.PhaseID != "PHS-1" {
		t.Errorf("got phase ID %q; want PHS-1", report.PhaseID)
	}
	if report.Total != 1 || report.Satisfied != 0 {
		t.Errorf("got %d/%d satisfied; want 0/1", report.Satisfied, report.Total)
	}
}

func TestValidate_PhaseSatisfactionNotInDefaultMappingChecks(t *testing.T) {
	mappingLayer := model.LayerMapping
	entities := []model.Entity{
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	}
	relations := []model.Relation{
		rel(1, "PHS-1", "REQ-1", model.RelationCovers),
	}

	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher(relations...)

	result, err := Validate(ValidateOptions{Layer: &mappingLayer}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, iss := range result.Issues {
		if iss.Check == "phase_satisfaction" {
			t.Errorf("phase_satisfaction should not run by default; got issue %+v", iss)
		}
	}
	if len(result.Satisfaction) != 0 {
		t.Errorf("phase_satisfaction reports should not be populated by default; got %+v", result.Satisfaction)
	}
}

type erroringRelationFetcher struct{}

func (erroringRelationFetcher) GetByEntity(string) ([]model.Relation, error) {
	return nil, errors.New("synthetic relation fetch failure")
}

func TestCheckPhaseSatisfaction_FetchErrorSurfacesAsIssue(t *testing.T) {
	phs1 := "PHS-1"
	ef := newMockEntityFetcher(
		execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil),
	)
	rf := erroringRelationFetcher{}

	issues, reports := checkPhaseSatisfaction(ValidateOptions{Phase: &phs1}, rf, ef)
	if len(issues) == 0 {
		t.Fatal("expected at least one issue when relation fetch fails")
	}
	if len(reports) != 0 {
		t.Errorf("expected no reports when fetch fails; got %d", len(reports))
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int)
	for _, s := range a {
		set[s]++
	}
	for _, s := range b {
		set[s]--
	}
	for _, count := range set {
		if count != 0 {
			return false
		}
	}
	return true
}

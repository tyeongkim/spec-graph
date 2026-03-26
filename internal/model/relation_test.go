package model

import "testing"

func TestRelationTypeConstants(t *testing.T) {
	expected := map[RelationType]string{
		RelationImplements:    "implements",
		RelationVerifies:      "verifies",
		RelationDependsOn:     "depends_on",
		RelationConstrainedBy: "constrained_by",
		RelationTriggers:      "triggers",
		RelationAnswers:       "answers",
		RelationAssumes:       "assumes",
		RelationHasCriterion:  "has_criterion",
		RelationMitigates:     "mitigates",
		RelationSupersedes:    "supersedes",
		RelationConflictsWith: "conflicts_with",
		RelationReferences:    "references",
		RelationBelongsTo:     "belongs_to",
		RelationPrecedes:      "precedes",
		RelationBlocks:        "blocks",
		RelationCovers:        "covers",
		RelationDelivers:      "delivers",
	}

	if len(expected) != 17 {
		t.Fatalf("expected 17 relation types, got %d", len(expected))
	}

	for rt, want := range expected {
		if string(rt) != want {
			t.Errorf("RelationType %q != %q", rt, want)
		}
	}
}

func TestIsEdgeAllowed(t *testing.T) {
	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		expected bool
	}{
		// implements: interface → requirement, criterion
		{"implements/interface→requirement", RelationImplements, EntityTypeInterface, EntityTypeRequirement, true},
		{"implements/interface→criterion", RelationImplements, EntityTypeInterface, EntityTypeCriterion, true},
		{"implements/test→requirement INVALID", RelationImplements, EntityTypeTest, EntityTypeRequirement, false},
		{"implements/interface→phase INVALID", RelationImplements, EntityTypeInterface, EntityTypePhase, false},

		// verifies: test → requirement, criterion, decision, interface, state
		{"verifies/test→requirement", RelationVerifies, EntityTypeTest, EntityTypeRequirement, true},
		{"verifies/test→criterion", RelationVerifies, EntityTypeTest, EntityTypeCriterion, true},
		{"verifies/test→decision", RelationVerifies, EntityTypeTest, EntityTypeDecision, true},
		{"verifies/test→interface", RelationVerifies, EntityTypeTest, EntityTypeInterface, true},
		{"verifies/test→state", RelationVerifies, EntityTypeTest, EntityTypeState, true},
		{"verifies/requirement→test INVALID", RelationVerifies, EntityTypeRequirement, EntityTypeTest, false},

		// depends_on: requirement,decision,interface,test,state → requirement,decision,interface,state,crosscut,assumption
		{"depends_on/requirement→decision", RelationDependsOn, EntityTypeRequirement, EntityTypeDecision, true},
		{"depends_on/test→assumption", RelationDependsOn, EntityTypeTest, EntityTypeAssumption, true},
		{"depends_on/state→interface", RelationDependsOn, EntityTypeState, EntityTypeInterface, true},
		{"depends_on/criterion→requirement INVALID", RelationDependsOn, EntityTypeCriterion, EntityTypeRequirement, false},
		{"depends_on/requirement→phase INVALID", RelationDependsOn, EntityTypeRequirement, EntityTypePhase, false},

		// constrained_by: requirement,decision,interface,state → crosscut,decision,assumption
		{"constrained_by/requirement→crosscut", RelationConstrainedBy, EntityTypeRequirement, EntityTypeCrosscut, true},
		{"constrained_by/state→assumption", RelationConstrainedBy, EntityTypeState, EntityTypeAssumption, true},
		{"constrained_by/test→crosscut INVALID", RelationConstrainedBy, EntityTypeTest, EntityTypeCrosscut, false},

		// triggers: interface,decision → state
		{"triggers/interface→state", RelationTriggers, EntityTypeInterface, EntityTypeState, true},
		{"triggers/decision→state", RelationTriggers, EntityTypeDecision, EntityTypeState, true},
		{"triggers/test→state INVALID", RelationTriggers, EntityTypeTest, EntityTypeState, false},
		{"triggers/interface→requirement INVALID", RelationTriggers, EntityTypeInterface, EntityTypeRequirement, false},

		// answers: decision → question
		{"answers/decision→question", RelationAnswers, EntityTypeDecision, EntityTypeQuestion, true},
		{"answers/requirement→question INVALID", RelationAnswers, EntityTypeRequirement, EntityTypeQuestion, false},
		{"answers/decision→decision INVALID", RelationAnswers, EntityTypeDecision, EntityTypeDecision, false},

		// assumes: requirement,decision,interface → assumption
		{"assumes/requirement→assumption", RelationAssumes, EntityTypeRequirement, EntityTypeAssumption, true},
		{"assumes/interface→assumption", RelationAssumes, EntityTypeInterface, EntityTypeAssumption, true},
		{"assumes/test→assumption INVALID", RelationAssumes, EntityTypeTest, EntityTypeAssumption, false},

		// has_criterion: requirement → criterion
		{"has_criterion/requirement→criterion", RelationHasCriterion, EntityTypeRequirement, EntityTypeCriterion, true},
		{"has_criterion/decision→criterion INVALID", RelationHasCriterion, EntityTypeDecision, EntityTypeCriterion, false},

		// mitigates: decision,test,crosscut → risk
		{"mitigates/decision→risk", RelationMitigates, EntityTypeDecision, EntityTypeRisk, true},
		{"mitigates/crosscut→risk", RelationMitigates, EntityTypeCrosscut, EntityTypeRisk, true},
		{"mitigates/requirement→risk INVALID", RelationMitigates, EntityTypeRequirement, EntityTypeRisk, false},

		// supersedes: same type only
		{"supersedes/req→req same type", RelationSupersedes, EntityTypeRequirement, EntityTypeRequirement, true},
		{"supersedes/dec→dec same type", RelationSupersedes, EntityTypeDecision, EntityTypeDecision, true},
		{"supersedes/req→dec diff type INVALID", RelationSupersedes, EntityTypeRequirement, EntityTypeDecision, false},

		// conflicts_with: any pair
		{"conflicts_with/req→dec any pair", RelationConflictsWith, EntityTypeRequirement, EntityTypeDecision, true},
		{"conflicts_with/test→risk any pair", RelationConflictsWith, EntityTypeTest, EntityTypeRisk, true},
		{"conflicts_with/phase→phase same", RelationConflictsWith, EntityTypePhase, EntityTypePhase, true},

		// references: any pair
		{"references/req→phase any pair", RelationReferences, EntityTypeRequirement, EntityTypePhase, true},
		{"references/risk→test any pair", RelationReferences, EntityTypeRisk, EntityTypeTest, true},
		{"references/criterion→criterion same", RelationReferences, EntityTypeCriterion, EntityTypeCriterion, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, nil)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q) = %v; want %v",
					tc.relType, tc.from, tc.to, got, tc.expected)
			}
		})
	}
}

func TestRelationStruct(t *testing.T) {
	r := Relation{
		ID:        1,
		FromID:    "REQ-001",
		ToID:      "DEC-001",
		Type:      RelationDependsOn,
		Layer:     LayerArch,
		Weight:    1.0,
		Metadata:  []byte(`{}`),
		CreatedAt: "2025-01-01T00:00:00Z",
	}

	if r.ID != 1 {
		t.Errorf("ID = %d; want 1", r.ID)
	}
	if r.FromID != "REQ-001" {
		t.Errorf("FromID = %q; want %q", r.FromID, "REQ-001")
	}
	if r.Type != RelationDependsOn {
		t.Errorf("Type = %q; want %q", r.Type, RelationDependsOn)
	}
	if r.Layer != LayerArch {
		t.Errorf("Layer = %q; want %q", r.Layer, LayerArch)
	}
	if r.Weight != 1.0 {
		t.Errorf("Weight = %f; want 1.0", r.Weight)
	}
}

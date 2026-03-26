package model

import "testing"

func TestLayerForEntityType(t *testing.T) {
	tests := []struct {
		name string
		et   EntityType
		want Layer
	}{
		{"requirement", EntityTypeRequirement, LayerArch},
		{"decision", EntityTypeDecision, LayerArch},
		{"interface", EntityTypeInterface, LayerArch},
		{"state", EntityTypeState, LayerArch},
		{"test", EntityTypeTest, LayerArch},
		{"crosscut", EntityTypeCrosscut, LayerArch},
		{"criterion", EntityTypeCriterion, LayerArch},
		{"assumption", EntityTypeAssumption, LayerArch},
		{"risk", EntityTypeRisk, LayerArch},
		{"question", EntityTypeQuestion, LayerArch},
		{"phase", EntityTypePhase, LayerExec},
		{"plan", EntityType("plan"), LayerExec},
		{"unknown", EntityType("nonexistent"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LayerForEntityType(tt.et)
			if got != tt.want {
				t.Errorf("LayerForEntityType(%q) = %q; want %q", tt.et, got, tt.want)
			}
		})
	}
}

func TestLayerForRelationType(t *testing.T) {
	tests := []struct {
		name string
		rt   RelationType
		want Layer
	}{
		{"implements", RelationImplements, LayerArch},
		{"verifies", RelationVerifies, LayerArch},
		{"depends_on", RelationDependsOn, LayerArch},
		{"constrained_by", RelationConstrainedBy, LayerArch},
		{"triggers", RelationTriggers, LayerArch},
		{"answers", RelationAnswers, LayerArch},
		{"assumes", RelationAssumes, LayerArch},
		{"has_criterion", RelationHasCriterion, LayerArch},
		{"mitigates", RelationMitigates, LayerArch},
		{"supersedes", RelationSupersedes, LayerArch},
		{"conflicts_with", RelationConflictsWith, LayerArch},
		{"references", RelationReferences, LayerArch},
		{"belongs_to", RelationType("belongs_to"), LayerExec},
		{"precedes", RelationType("precedes"), LayerExec},
		{"blocks", RelationType("blocks"), LayerExec},
		{"covers", RelationType("covers"), LayerMapping},
		{"delivers", RelationType("delivers"), LayerMapping},
		{"unknown", RelationType("nonexistent"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LayerForRelationType(tt.rt)
			if got != tt.want {
				t.Errorf("LayerForRelationType(%q) = %q; want %q", tt.rt, got, tt.want)
			}
		})
	}
}

func TestIsValidLayer(t *testing.T) {
	tests := []struct {
		name string
		l    Layer
		want bool
	}{
		{"arch", LayerArch, true},
		{"exec", LayerExec, true},
		{"mapping", LayerMapping, true},
		{"empty", "", false},
		{"invalid", Layer("other"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidLayer(tt.l)
			if got != tt.want {
				t.Errorf("IsValidLayer(%q) = %v; want %v", tt.l, got, tt.want)
			}
		})
	}
}

func TestValidLayersCompleteness(t *testing.T) {
	expected := map[Layer]bool{LayerArch: true, LayerExec: true, LayerMapping: true}
	if len(ValidLayers) != len(expected) {
		t.Fatalf("ValidLayers has %d entries; want %d", len(ValidLayers), len(expected))
	}
	for _, l := range ValidLayers {
		if !expected[l] {
			t.Errorf("unexpected layer in ValidLayers: %q", l)
		}
	}
}

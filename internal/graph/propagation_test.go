package graph

import (
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

var allRelationTypes = []model.RelationType{
	model.RelationImplements,
	model.RelationVerifies,
	model.RelationDependsOn,
	model.RelationConstrainedBy,
	model.RelationPlannedIn,
	model.RelationDeliveredIn,
	model.RelationTriggers,
	model.RelationAnswers,
	model.RelationAssumes,
	model.RelationHasCriterion,
	model.RelationMitigates,
	model.RelationSupersedes,
	model.RelationConflictsWith,
	model.RelationReferences,
}

func TestPropagationTableCompleteness(t *testing.T) {
	if len(PropagationTable) != 14 {
		t.Errorf("PropagationTable has %d entries; want 14", len(PropagationTable))
	}
	for _, rt := range allRelationTypes {
		if _, ok := PropagationTable[rt]; !ok {
			t.Errorf("PropagationTable missing entry for %q", rt)
		}
	}
}

func TestPropagationTableNonZeroScores(t *testing.T) {
	for _, rt := range allRelationTypes {
		rule, ok := PropagationTable[rt]
		if !ok {
			continue
		}
		s := rule.Scores
		if s.Structural == 0 && s.Behavioral == 0 && s.Planning == 0 {
			t.Errorf("PropagationTable[%q] has all-zero scores", rt)
		}
	}
}

func TestScoreToSeverity(t *testing.T) {
	tests := []struct {
		score float64
		want  Severity
	}{
		{0.9, SeverityHigh},
		{0.7, SeverityHigh},
		{0.5, SeverityMedium},
		{0.4, SeverityMedium},
		{0.2, SeverityLow},
		{0.01, SeverityLow},
		{0.0, ""},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := ScoreToSeverity(tt.score)
			if got != tt.want {
				t.Errorf("ScoreToSeverity(%v) = %q; want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestOverallSeverity(t *testing.T) {
	tests := []struct {
		scores DimensionScores
		want   Severity
	}{
		{DimensionScores{0.9, 0.1, 0.1}, SeverityHigh},
		{DimensionScores{0.1, 0.5, 0.1}, SeverityMedium},
		{DimensionScores{0.1, 0.1, 0.2}, SeverityLow},
		{DimensionScores{0.0, 0.0, 0.0}, ""},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := OverallSeverity(tt.scores)
			if got != tt.want {
				t.Errorf("OverallSeverity(%+v) = %q; want %q", tt.scores, got, tt.want)
			}
		})
	}
}

func TestReverseWeakFactor(t *testing.T) {
	if ReverseWeakFactor != 0.5 {
		t.Errorf("ReverseWeakFactor = %v; want 0.5", ReverseWeakFactor)
	}
}

func TestReasonTemplates(t *testing.T) {
	if len(ReasonTemplates) != 14 {
		t.Errorf("ReasonTemplates has %d entries; want 14", len(ReasonTemplates))
	}
	for _, rt := range allRelationTypes {
		reason, ok := ReasonTemplates[rt]
		if !ok {
			t.Errorf("ReasonTemplates missing entry for %q", rt)
			continue
		}
		if reason == "" {
			t.Errorf("ReasonTemplates[%q] is empty", rt)
		}
	}
}

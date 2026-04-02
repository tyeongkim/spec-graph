// Package graph implements impact propagation semantics for the spec-graph.
package graph

import "github.com/tyeongkim/spec-graph/internal/model"

// PropagationDirection describes how impact flows along a relation edge.
type PropagationDirection string

const (
	// Forward means impact flows from source to target only.
	Forward PropagationDirection = "forward"
	// Bidirectional means impact flows in both directions equally.
	Bidirectional PropagationDirection = "bidirectional"
	// ForwardReverseWeak means impact flows forward at full weight and
	// reverse at ReverseWeakFactor of the original weight.
	ForwardReverseWeak PropagationDirection = "forward_reverse_weak"
)

// ReverseWeakFactor is the multiplier applied to scores when propagating
// in the reverse direction for ForwardReverseWeak relations.
const ReverseWeakFactor float64 = 0.5

// Severity thresholds for converting a numeric score to a severity level.
const (
	ThresholdHigh   float64 = 0.7
	ThresholdMedium float64 = 0.4
)

// Severity represents the impact severity level of a propagated change.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// DimensionScores holds per-dimension propagation weights for a relation type.
type DimensionScores struct {
	Structural float64
	Behavioral float64
	Planning   float64
}

// PropagationRule defines how impact propagates along a specific relation type.
type PropagationRule struct {
	Direction PropagationDirection
	Scores    DimensionScores
	Note      string
}

// PropagationTable maps each RelationType to its propagation rule.
// Weights are derived from the spec-graph PLAN.md propagation weight table.
var PropagationTable = map[model.RelationType]PropagationRule{
	model.RelationImplements: {
		Direction: Bidirectional,
		Scores:    DimensionScores{Structural: 0.9, Behavioral: 0.8, Planning: 0.4},
	},
	model.RelationVerifies: {
		Direction: ForwardReverseWeak,
		Scores:    DimensionScores{Structural: 0.4, Behavioral: 0.8, Planning: 0.3},
	},
	model.RelationDependsOn: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.8, Behavioral: 0.7, Planning: 0.4},
	},
	model.RelationConstrainedBy: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.5, Behavioral: 0.8, Planning: 0.4},
	},
	model.RelationTriggers: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.6, Behavioral: 0.9, Planning: 0.2},
	},
	model.RelationAnswers: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.2, Behavioral: 0.7, Planning: 0.3},
	},
	model.RelationAssumes: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.3, Behavioral: 0.8, Planning: 0.5},
	},
	model.RelationHasCriterion: {
		Direction: Bidirectional,
		Scores:    DimensionScores{Structural: 0.3, Behavioral: 0.9, Planning: 0.2},
	},
	model.RelationMitigates: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.2, Behavioral: 0.6, Planning: 0.4},
	},
	model.RelationSupersedes: {
		Direction: ForwardReverseWeak,
		Scores:    DimensionScores{Structural: 0.4, Behavioral: 0.5, Planning: 0.3},
	},
	model.RelationConflictsWith: {
		Direction: Bidirectional,
		Scores:    DimensionScores{Structural: 0.8, Behavioral: 0.9, Planning: 0.5},
	},
	model.RelationReferences: {
		Direction: Bidirectional,
		Scores:    DimensionScores{Structural: 0.1, Behavioral: 0.1, Planning: 0.1},
		Note:      "bidirectional weak",
	},
	model.RelationCovers: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.1, Behavioral: 0.2, Planning: 0.8},
	},
	model.RelationDelivers: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.3, Behavioral: 0.3, Planning: 0.9},
	},
	model.RelationBelongsTo: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.1, Behavioral: 0.1, Planning: 0.7},
	},
	model.RelationPrecedes: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.1, Behavioral: 0.1, Planning: 0.6},
	},
	model.RelationBlocks: {
		Direction: Forward,
		Scores:    DimensionScores{Structural: 0.2, Behavioral: 0.2, Planning: 0.8},
	},
}

// ReasonTemplates maps each RelationType to a human-readable reason string
// used when explaining why an entity is impacted.
var ReasonTemplates = map[model.RelationType]string{
	model.RelationImplements:    "direct implementation",
	model.RelationVerifies:      "verification dependency",
	model.RelationDependsOn:     "direct dependency",
	model.RelationConstrainedBy: "constraint dependency",
	model.RelationTriggers:      "state transition trigger",
	model.RelationAnswers:       "question resolution",
	model.RelationAssumes:       "assumption dependency",
	model.RelationHasCriterion:  "acceptance criterion",
	model.RelationMitigates:     "risk mitigation",
	model.RelationSupersedes:    "superseded entity",
	model.RelationConflictsWith: "semantic conflict",
	model.RelationReferences:    "weak reference",
	model.RelationCovers:        "coverage mapping",
	model.RelationDelivers:      "delivery mapping",
	model.RelationBelongsTo:     "phase membership",
	model.RelationPrecedes:      "phase ordering",
	model.RelationBlocks:        "blocking dependency",
}

// ScoreToSeverity converts a numeric propagation score to a Severity level.
// Returns SeverityHigh for score >= ThresholdHigh, SeverityMedium for >= ThresholdMedium,
// SeverityLow for any positive score, and empty string for zero.
func ScoreToSeverity(score float64) Severity {
	switch {
	case score >= ThresholdHigh:
		return SeverityHigh
	case score >= ThresholdMedium:
		return SeverityMedium
	case score > 0.0:
		return SeverityLow
	default:
		return ""
	}
}

// OverallSeverity returns the highest severity across all three dimensions.
func OverallSeverity(scores DimensionScores) Severity {
	max := scores.Structural
	if scores.Behavioral > max {
		max = scores.Behavioral
	}
	if scores.Planning > max {
		max = scores.Planning
	}
	return ScoreToSeverity(max)
}

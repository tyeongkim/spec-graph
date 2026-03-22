package model

import "encoding/json"

type RelationType string

const (
	RelationImplements    RelationType = "implements"
	RelationVerifies      RelationType = "verifies"
	RelationDependsOn     RelationType = "depends_on"
	RelationConstrainedBy RelationType = "constrained_by"
	RelationPlannedIn     RelationType = "planned_in"
	RelationDeliveredIn   RelationType = "delivered_in"
	RelationTriggers      RelationType = "triggers"
	RelationAnswers       RelationType = "answers"
	RelationAssumes       RelationType = "assumes"
	RelationHasCriterion  RelationType = "has_criterion"
	RelationMitigates     RelationType = "mitigates"
	RelationSupersedes    RelationType = "supersedes"
	RelationConflictsWith RelationType = "conflicts_with"
	RelationReferences    RelationType = "references"
)

type Relation struct {
	ID        int             `json:"id"`
	FromID    string          `json:"from_id"`
	ToID      string          `json:"to_id"`
	Type      RelationType    `json:"type"`
	Weight    float64         `json:"weight"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt string          `json:"created_at"`
}

type edgeRule struct {
	From []EntityType
	To   []EntityType
}

var edgeMatrix = map[RelationType]edgeRule{
	RelationImplements: {
		From: []EntityType{EntityTypeInterface},
		To:   []EntityType{EntityTypeRequirement, EntityTypeCriterion},
	},
	RelationVerifies: {
		From: []EntityType{EntityTypeTest},
		To:   []EntityType{EntityTypeRequirement, EntityTypeCriterion, EntityTypeDecision, EntityTypeInterface, EntityTypeState},
	},
	RelationDependsOn: {
		From: []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypeInterface, EntityTypePhase, EntityTypeTest, EntityTypeState},
		To:   []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypeInterface, EntityTypeState, EntityTypeCrosscut, EntityTypeAssumption},
	},
	RelationConstrainedBy: {
		From: []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypeInterface, EntityTypePhase, EntityTypeState},
		To:   []EntityType{EntityTypeCrosscut, EntityTypeDecision, EntityTypeAssumption},
	},
	RelationPlannedIn: {
		From: []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypeInterface, EntityTypeTest, EntityTypeQuestion, EntityTypeRisk, EntityTypeCriterion},
		To:   []EntityType{EntityTypePhase},
	},
	RelationDeliveredIn: {
		From: []EntityType{EntityTypeInterface, EntityTypeState, EntityTypeTest, EntityTypeDecision},
		To:   []EntityType{EntityTypePhase},
	},
	RelationTriggers: {
		From: []EntityType{EntityTypeInterface, EntityTypeDecision},
		To:   []EntityType{EntityTypeState},
	},
	RelationAnswers: {
		From: []EntityType{EntityTypeDecision},
		To:   []EntityType{EntityTypeQuestion},
	},
	RelationAssumes: {
		From: []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypePhase, EntityTypeInterface},
		To:   []EntityType{EntityTypeAssumption},
	},
	RelationHasCriterion: {
		From: []EntityType{EntityTypeRequirement},
		To:   []EntityType{EntityTypeCriterion},
	},
	RelationMitigates: {
		From: []EntityType{EntityTypeDecision, EntityTypeTest, EntityTypeCrosscut, EntityTypePhase},
		To:   []EntityType{EntityTypeRisk},
	},
}

func IsEdgeAllowed(relType RelationType, fromEntityType, toEntityType EntityType) bool {
	switch relType {
	case RelationSupersedes:
		return fromEntityType == toEntityType
	case RelationConflictsWith, RelationReferences:
		return true
	}

	rule, ok := edgeMatrix[relType]
	if !ok {
		return false
	}

	return contains(rule.From, fromEntityType) && contains(rule.To, toEntityType)
}

func contains(types []EntityType, t EntityType) bool {
	for _, v := range types {
		if v == t {
			return true
		}
	}
	return false
}

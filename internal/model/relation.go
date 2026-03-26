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

	// Execution layer relations
	RelationBelongsTo RelationType = "belongs_to"
	RelationPrecedes  RelationType = "precedes"
	RelationBlocks    RelationType = "blocks"

	// Mapping layer relations
	RelationCovers   RelationType = "covers"
	RelationDelivers RelationType = "delivers"
)

var ValidRelationTypes = []RelationType{
	RelationImplements,
	RelationVerifies,
	RelationDependsOn,
	RelationConstrainedBy,
	RelationPlannedIn,
	RelationDeliveredIn,
	RelationTriggers,
	RelationAnswers,
	RelationAssumes,
	RelationHasCriterion,
	RelationMitigates,
	RelationSupersedes,
	RelationConflictsWith,
	RelationReferences,
	RelationBelongsTo,
	RelationPrecedes,
	RelationBlocks,
	RelationCovers,
	RelationDelivers,
}

type Relation struct {
	ID        int             `json:"id"`
	FromID    string          `json:"from_id"`
	ToID      string          `json:"to_id"`
	Type      RelationType    `json:"type"`
	Layer     Layer           `json:"layer"`
	Weight    float64         `json:"weight"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt string          `json:"created_at"`
}

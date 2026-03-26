package model

import "slices"

// Layer represents the architectural layer an entity or relation belongs to.
// The three layers are: arch (architecture), exec (execution), and mapping (cross-layer links).
type Layer string

const (
	// LayerArch is the architecture layer containing requirements, decisions, interfaces, etc.
	LayerArch Layer = "arch"
	// LayerExec is the execution layer containing plans and phases.
	LayerExec Layer = "exec"
	// LayerMapping is the mapping layer containing cross-layer relations like covers and delivers.
	LayerMapping Layer = "mapping"
)

// ValidLayers contains all valid layer values.
var ValidLayers = []Layer{LayerArch, LayerExec, LayerMapping}

var typeLayerMap = map[EntityType]Layer{
	EntityTypeRequirement: LayerArch,
	EntityTypeDecision:    LayerArch,
	EntityTypeInterface:   LayerArch,
	EntityTypeState:       LayerArch,
	EntityTypeTest:        LayerArch,
	EntityTypeCrosscut:    LayerArch,
	EntityTypeCriterion:   LayerArch,
	EntityTypeAssumption:  LayerArch,
	EntityTypeRisk:        LayerArch,
	EntityTypeQuestion:    LayerArch,

	EntityTypePhase: LayerExec,
	EntityTypePlan:  LayerExec,
}

// relationLayerMap classifies each relation type into a layer.
// RelationReferences is classified as arch but is allowed cross-layer (design decision Q1).
var relationLayerMap = map[RelationType]Layer{
	RelationImplements:    LayerArch,
	RelationVerifies:      LayerArch,
	RelationDependsOn:     LayerArch,
	RelationConstrainedBy: LayerArch,
	RelationTriggers:      LayerArch,
	RelationAnswers:       LayerArch,
	RelationAssumes:       LayerArch,
	RelationHasCriterion:  LayerArch,
	RelationMitigates:     LayerArch,
	RelationSupersedes:    LayerArch,
	RelationConflictsWith: LayerArch,
	RelationReferences:    LayerArch,

	RelationBelongsTo: LayerExec,
	RelationPrecedes:  LayerExec,
	RelationBlocks:    LayerExec,

	RelationPlannedIn:   LayerMapping,
	RelationDeliveredIn: LayerMapping,
	RelationCovers:      LayerMapping,
	RelationDelivers:    LayerMapping,
}

// LayerForEntityType returns the layer for the given entity type.
// Returns an empty string if the entity type is unknown.
func LayerForEntityType(t EntityType) Layer {
	return typeLayerMap[t]
}

// LayerForRelationType returns the layer for the given relation type.
// Returns an empty string if the relation type is unknown.
func LayerForRelationType(t RelationType) Layer {
	return relationLayerMap[t]
}

// IsValidLayer reports whether l is a recognized layer value.
func IsValidLayer(l Layer) bool {
	return slices.Contains(ValidLayers, l)
}

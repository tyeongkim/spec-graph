package model

import "slices"

// edgeRule defines the allowed source and target entity types for a relation.
type edgeRule struct {
	From []EntityType
	To   []EntityType
}

// archEdgeMatrix defines edge rules for the architecture layer.
// NOTE: planned_in and delivered_in remain here until Phase 6 cleanup.
// NOTE: phase remains in From lists for depends_on/constrained_by/assumes/mitigates for backward compat.
var archEdgeMatrix = map[RelationType]edgeRule{
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

// execEdgeMatrix defines edge rules for the execution layer.
var execEdgeMatrix = map[RelationType]edgeRule{
	RelationBelongsTo: {
		From: []EntityType{EntityTypePhase},
		To:   []EntityType{EntityTypePlan},
	},
	RelationPrecedes: {
		From: []EntityType{EntityTypePhase},
		To:   []EntityType{EntityTypePhase},
	},
	RelationBlocks: {
		From: []EntityType{EntityTypePhase},
		To:   []EntityType{EntityTypePhase},
	},
}

// mappingEdgeMatrix defines edge rules for the mapping (cross-layer) layer.
var mappingEdgeMatrix = map[RelationType]edgeRule{
	RelationCovers: {
		From: []EntityType{EntityTypePhase},
		To:   []EntityType{EntityTypeRequirement, EntityTypeDecision, EntityTypeInterface, EntityTypeTest, EntityTypeQuestion, EntityTypeRisk, EntityTypeCriterion, EntityTypeAssumption},
	},
	RelationDelivers: {
		From: []EntityType{EntityTypePhase},
		To:   []EntityType{EntityTypeRequirement, EntityTypeInterface, EntityTypeState, EntityTypeTest, EntityTypeDecision, EntityTypeCriterion},
	},
}

// IsEdgeAllowed reports whether a relation of the given type is permitted
// between the specified entity types.
//
// When layer is nil, all matrices (arch, exec, mapping) are checked along with
// special-case relations (supersedes, conflicts_with, references).
// When layer is non-nil, only the matrix for that specific layer is consulted.
func IsEdgeAllowed(relType RelationType, fromEntityType, toEntityType EntityType, layer *Layer) bool {
	// Special cases apply regardless of layer filter.
	switch relType {
	case RelationSupersedes:
		return fromEntityType == toEntityType
	case RelationConflictsWith:
		// Any→any for backward compat; will be restricted to arch-only in Phase 6.
		return true
	case RelationReferences:
		// Any→any cross-layer per Q1 decision.
		return true
	}

	if layer == nil {
		return checkAllMatrices(relType, fromEntityType, toEntityType)
	}
	return checkMatrix(matrixForLayer(*layer), relType, fromEntityType, toEntityType)
}

// checkAllMatrices checks the relation against arch, exec, and mapping matrices.
func checkAllMatrices(relType RelationType, from, to EntityType) bool {
	if checkMatrix(archEdgeMatrix, relType, from, to) {
		return true
	}
	if checkMatrix(execEdgeMatrix, relType, from, to) {
		return true
	}
	return checkMatrix(mappingEdgeMatrix, relType, from, to)
}

// checkMatrix checks a single matrix for the given relation and entity types.
func checkMatrix(matrix map[RelationType]edgeRule, relType RelationType, from, to EntityType) bool {
	rule, ok := matrix[relType]
	if !ok {
		return false
	}
	return contains(rule.From, from) && contains(rule.To, to)
}

// matrixForLayer returns the edge matrix corresponding to the given layer.
func matrixForLayer(l Layer) map[RelationType]edgeRule {
	switch l {
	case LayerArch:
		return archEdgeMatrix
	case LayerExec:
		return execEdgeMatrix
	case LayerMapping:
		return mappingEdgeMatrix
	default:
		return nil
	}
}

// contains reports whether the slice contains the given entity type.
func contains(types []EntityType, t EntityType) bool {
	return slices.Contains(types, t)
}

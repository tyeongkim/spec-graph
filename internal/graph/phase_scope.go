package graph

import (
	"fmt"
	"sort"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// EffectivePhaseScopeResult is the canonical architecture scope and delivery for a phase.
type EffectivePhaseScopeResult struct {
	Covered     []string
	Delivered   []string
	Relations   []model.Relation
	TaskIDs     []string
	TaskManaged bool
}

// EffectivePhaseScope derives task mappings when tasks belong to the phase,
// otherwise preserving direct phase mappings for legacy taskless phases.
func EffectivePhaseScope(phaseID string, rf RelationFetcher) (EffectivePhaseScopeResult, error) {
	phaseRelations, err := rf.GetByEntity(phaseID)
	if err != nil {
		return EffectivePhaseScopeResult{}, fmt.Errorf("fetch phase relations: %w", err)
	}

	taskIDs := make([]string, 0)
	for _, relation := range phaseRelations {
		if relation.Type == model.RelationBelongsTo && relation.ToID == phaseID {
			taskIDs = append(taskIDs, relation.FromID)
		}
	}
	sort.Strings(taskIDs)

	if len(taskIDs) == 0 {
		return scopeFromSources(phaseID, nil, phaseRelations), nil
	}

	result := EffectivePhaseScopeResult{TaskIDs: taskIDs, TaskManaged: true}
	for _, taskID := range taskIDs {
		taskRelations, fetchErr := rf.GetByEntity(taskID)
		if fetchErr != nil {
			return EffectivePhaseScopeResult{}, fmt.Errorf("fetch task %s relations: %w", taskID, fetchErr)
		}
		result = mergeSourceMappings(result, taskID, taskRelations)
	}
	result.Covered = sortedKeys(result.Covered)
	result.Delivered = sortedKeys(result.Delivered)
	sortRelations(result.Relations)
	return result, nil
}

func scopeFromSources(sourceID string, taskIDs []string, relations []model.Relation) EffectivePhaseScopeResult {
	result := EffectivePhaseScopeResult{TaskIDs: taskIDs}
	covered := make(map[string]struct{})
	delivered := make(map[string]struct{})
	for _, relation := range relations {
		if relation.FromID != sourceID {
			continue
		}
		switch relation.Type {
		case model.RelationCovers:
			if _, exists := covered[relation.ToID]; !exists {
				covered[relation.ToID] = struct{}{}
				result.Covered = append(result.Covered, relation.ToID)
			}
			result.Relations = append(result.Relations, relation)
		case model.RelationDelivers:
			if _, exists := delivered[relation.ToID]; !exists {
				delivered[relation.ToID] = struct{}{}
				result.Delivered = append(result.Delivered, relation.ToID)
			}
			result.Relations = append(result.Relations, relation)
		}
	}
	return result
}

func mergeSourceMappings(result EffectivePhaseScopeResult, sourceID string, relations []model.Relation) EffectivePhaseScopeResult {
	covered := sliceSet(result.Covered)
	delivered := sliceSet(result.Delivered)
	for _, relation := range relations {
		if relation.FromID != sourceID {
			continue
		}
		switch relation.Type {
		case model.RelationCovers:
			covered[relation.ToID] = struct{}{}
			result.Relations = append(result.Relations, relation)
		case model.RelationDelivers:
			delivered[relation.ToID] = struct{}{}
			result.Relations = append(result.Relations, relation)
		}
	}
	result.Covered = mapKeys(covered)
	result.Delivered = mapKeys(delivered)
	return result
}

func sliceSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func sortedKeys(ids []string) []string {
	sort.Strings(ids)
	return ids
}

func mapKeys(set map[string]struct{}) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

func sortRelations(relations []model.Relation) {
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].FromID != relations[j].FromID {
			return relations[i].FromID < relations[j].FromID
		}
		if relations[i].Type != relations[j].Type {
			return relations[i].Type < relations[j].Type
		}
		return relations[i].ToID < relations[j].ToID
	})
}

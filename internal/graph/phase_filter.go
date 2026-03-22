package graph

import (
	"github.com/taeyeong/spec-graph/internal/model"
)

type phaseFilteredEntityFetcher struct {
	inner   EntityFetcher
	allowed map[string]bool
}

func newPhaseFilteredEntityFetcher(inner EntityFetcher, rf RelationFetcher, phaseID string) (*phaseFilteredEntityFetcher, error) {
	allowed := map[string]bool{phaseID: true}

	rels, err := rf.GetByEntity(phaseID)
	if err != nil {
		return nil, err
	}
	for _, r := range rels {
		if (r.Type == model.RelationPlannedIn || r.Type == model.RelationDeliveredIn) && r.ToID == phaseID {
			allowed[r.FromID] = true
		}
	}

	return &phaseFilteredEntityFetcher{inner: inner, allowed: allowed}, nil
}

func (f *phaseFilteredEntityFetcher) Get(id string) (model.Entity, error) {
	return f.inner.Get(id)
}

func (f *phaseFilteredEntityFetcher) List(filters EntityListFilters) ([]model.Entity, error) {
	all, err := f.inner.List(filters)
	if err != nil {
		return nil, err
	}
	var result []model.Entity
	for _, e := range all {
		if f.allowed[e.ID] {
			result = append(result, e)
		}
	}
	return result, nil
}

type phaseFilteredRelationFetcher struct {
	inner   RelationFetcher
	allowed map[string]bool
}

func newPhaseFilteredRelationFetcher(inner RelationFetcher, allowed map[string]bool) *phaseFilteredRelationFetcher {
	return &phaseFilteredRelationFetcher{inner: inner, allowed: allowed}
}

func (f *phaseFilteredRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	if !f.allowed[entityID] {
		return nil, nil
	}
	all, err := f.inner.GetByEntity(entityID)
	if err != nil {
		return nil, err
	}
	var result []model.Relation
	for _, r := range all {
		if f.allowed[r.FromID] && f.allowed[r.ToID] {
			result = append(result, r)
		}
	}
	return result, nil
}

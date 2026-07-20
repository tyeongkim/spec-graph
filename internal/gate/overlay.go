package gate

import (
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type overlayEntityFetcher struct {
	base      validate.EntityFetcher
	candidate model.Entity
}

func newOverlayEntityFetcher(base validate.EntityFetcher, candidate model.Entity) validate.EntityFetcher {
	if candidate.ID == "" {
		return base
	}
	return &overlayEntityFetcher{base: base, candidate: candidate}
}

func (f *overlayEntityFetcher) Get(id string) (model.Entity, error) {
	if id == f.candidate.ID {
		return f.candidate, nil
	}
	return f.base.Get(id)
}

func (f *overlayEntityFetcher) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	entities, err := f.base.List(filters)
	if err != nil {
		return nil, err
	}
	result := make([]model.Entity, 0, len(entities)+1)
	for _, entity := range entities {
		if entity.ID != f.candidate.ID {
			result = append(result, entity)
		}
	}
	if matchesFilters(f.candidate, filters) {
		result = append(result, f.candidate)
	}
	return result, nil
}

func matchesFilters(entity model.Entity, filters validate.EntityListFilters) bool {
	return (filters.Type == nil || entity.Type == *filters.Type) &&
		(filters.Status == nil || entity.Status == *filters.Status) &&
		(filters.Layer == nil || entity.Layer == *filters.Layer)
}

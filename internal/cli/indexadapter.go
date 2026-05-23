package cli

import (
	"encoding/json"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type indexEntityFetcher struct {
	idx *index.Index
}

func (f *indexEntityFetcher) Get(id string) (model.Entity, error) {
	rec, err := f.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return entityFromRecord(rec), nil
}

func (f *indexEntityFetcher) List(filters graph.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	recs, err := f.idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	return entitiesToModel(recs), nil
}

type indexValidateEntityFetcher struct {
	idx *index.Index
}

func (f *indexValidateEntityFetcher) Get(id string) (model.Entity, error) {
	rec, err := f.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return entityFromRecord(rec), nil
}

func (f *indexValidateEntityFetcher) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	if filters.Layer != nil {
		ef.Layer = string(*filters.Layer)
	}
	recs, err := f.idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	return entitiesToModel(recs), nil
}

type indexRelationFetcher struct {
	idx *index.Index
}

func (f *indexRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	recs, err := f.idx.GetRelationsByEntity(entityID)
	if err != nil {
		return nil, err
	}
	return relationsToModel(recs), nil
}

func entityFromRecord(rec *index.EntityRecord) model.Entity {
	return model.Entity{
		ID:          rec.ID,
		Type:        model.EntityType(rec.Type),
		Layer:       model.Layer(rec.Layer),
		Status:      model.EntityStatus(rec.Status),
		Title:       rec.Title,
		Description: rec.Description,
		Metadata:    json.RawMessage(rec.Metadata),
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

func entitiesToModel(recs []index.EntityRecord) []model.Entity {
	entities := make([]model.Entity, len(recs))
	for i := range recs {
		entities[i] = entityFromRecord(&recs[i])
	}
	return entities
}

func relationFromRecord(rec *index.RelationRecord) model.Relation {
	return model.Relation{
		FromID:   rec.FromID,
		ToID:     rec.ToID,
		Type:     model.RelationType(rec.Type),
		Layer:    model.Layer(rec.Layer),
		Weight:   rec.Weight,
		Metadata: json.RawMessage(rec.Metadata),
	}
}

func relationsToModel(recs []index.RelationRecord) []model.Relation {
	rels := make([]model.Relation, len(recs))
	for i := range recs {
		rels[i] = relationFromRecord(&recs[i])
	}
	return rels
}

func listEntitiesFromIndex(idx *index.Index, layer *model.Layer) ([]model.Entity, error) {
	var ef index.EntityFilters
	if layer != nil {
		ef.Layer = string(*layer)
	}
	recs, err := idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	return entitiesToModel(recs), nil
}

func listRelationsFromIndex(idx *index.Index, layer *model.Layer) ([]model.Relation, error) {
	var rf index.RelationFilters
	if layer != nil {
		rf.Layer = string(*layer)
	}
	recs, err := idx.ListRelations(rf)
	if err != nil {
		return nil, err
	}
	return relationsToModel(recs), nil
}

package specgraph

import (
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type stagedEntityFetcher struct {
	tx *txn
}

func (f *stagedEntityFetcher) Get(id string) (model.Entity, error) {
	if entry, ok := f.tx.staged[id]; ok {
		if entry.deleted {
			return model.Entity{}, &model.ErrEntityNotFound{ID: id}
		}
		if entry.file != nil {
			return entry.file.ToEntity()
		}
	}
	rec, err := f.tx.eng.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return engineEntityFromRecord(rec), nil
}

func (f *stagedEntityFetcher) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	indexed, err := (&engineEntityFetcher{idx: f.tx.eng.idx}).List(filters)
	if err != nil {
		return nil, err
	}

	entities := make([]model.Entity, 0, len(indexed)+len(f.tx.staged))
	for _, entity := range indexed {
		if _, staged := f.tx.staged[entity.ID]; !staged {
			entities = append(entities, entity)
		}
	}

	for _, entry := range f.tx.staged {
		if entry.deleted || entry.file == nil {
			continue
		}
		entity, convErr := entry.file.ToEntity()
		if convErr != nil {
			return nil, convErr
		}
		if validate.MatchesEntityFilters(entity, filters) {
			entities = append(entities, entity)
		}
	}
	return entities, nil
}

// stagedRelationFetcher resolves relations against staged state. Relations live
// as outbound entries inside entity files, so for a staged owner the index still
// holds that owner's previous outbound set; those entries are replaced rather
// than merged, because merging would resurrect relations the transaction
// deleted.
type stagedRelationFetcher struct {
	tx *txn
}

func (f *stagedRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	indexed, err := (&engineRelationFetcher{idx: f.tx.eng.idx}).GetByEntity(entityID)
	if err != nil {
		return nil, err
	}

	relations := make([]model.Relation, 0, len(indexed))
	for _, relation := range indexed {
		if _, staged := f.tx.staged[relation.FromID]; !staged {
			relations = append(relations, relation)
		}
	}

	for _, entry := range f.tx.staged {
		if entry.deleted || entry.file == nil {
			continue
		}
		outbound, convErr := entry.file.ToRelations()
		if convErr != nil {
			return nil, convErr
		}
		for _, relation := range outbound {
			if relation.FromID == entityID || relation.ToID == entityID {
				relations = append(relations, relation)
			}
		}
	}
	return relations, nil
}

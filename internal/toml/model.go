package spectoml

import (
	"encoding/json"
	"time"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// EntityFile represents the TOML structure for a single entity file.
// Layer is NOT stored — it is derived from entity type at load time.
// Timestamps are NOT stored — they are derived from git/filesystem.
type EntityFile struct {
	Schema      int              `toml:"schema"`
	ID          string           `toml:"id"`
	Type        model.EntityType `toml:"type"`
	Title       string           `toml:"title"`
	Description string           `toml:"description,omitempty"`
	Status      model.EntityStatus `toml:"status"`
	Metadata    map[string]any   `toml:"metadata,omitempty"`
	Relations   []RelationEntry  `toml:"relations,omitempty"`
}

// RelationEntry represents a single relation within an entity's TOML file.
// Weight is omitted if it equals the default (1.0).
// Metadata is included as an inline table if non-empty.
type RelationEntry struct {
	To       string         `toml:"to"`
	Type     model.RelationType `toml:"type"`
	Weight   float64        `toml:"weight,omitempty"`
	Metadata map[string]any `toml:"metadata,omitempty"`
}

// HistoryFile represents the TOML structure for an entity's history file.
type HistoryFile struct {
	EntityID string         `toml:"entity_id"`
	Entries  []HistoryEntry `toml:"entries"`
}

// HistoryEntry represents a single history entry in the history TOML file.
type HistoryEntry struct {
	Action    model.HistoryAction `toml:"action"`
	Reason    string              `toml:"reason"`
	Actor     string              `toml:"actor"`
	Detail    string              `toml:"detail,omitempty"`
	Timestamp time.Time           `toml:"timestamp"`
}

// ToEntity converts an EntityFile to a model.Entity.
// Layer is derived from the entity type. Timestamps are left empty
// (to be populated by the caller from git/filesystem).
func (ef *EntityFile) ToEntity() (model.Entity, error) {
	var meta json.RawMessage
	if len(ef.Metadata) > 0 {
		b, err := json.Marshal(ef.Metadata)
		if err != nil {
			return model.Entity{}, err
		}
		meta = b
	}

	return model.Entity{
		ID:          ef.ID,
		Type:        ef.Type,
		Layer:       model.LayerForEntityType(ef.Type),
		Title:       ef.Title,
		Description: ef.Description,
		Status:      ef.Status,
		Metadata:    meta,
	}, nil
}

// ToRelations converts the embedded RelationEntry slice to model.Relation values.
// The FromID is set to the entity file's ID.
func (ef *EntityFile) ToRelations() ([]model.Relation, error) {
	relations := make([]model.Relation, 0, len(ef.Relations))
	for _, re := range ef.Relations {
		var meta json.RawMessage
		if len(re.Metadata) > 0 {
			b, err := json.Marshal(re.Metadata)
			if err != nil {
				return nil, err
			}
			meta = b
		}

		weight := re.Weight
		if weight == 0 {
			weight = 1.0
		}

		relations = append(relations, model.Relation{
			FromID:   ef.ID,
			ToID:     re.To,
			Type:     re.Type,
			Layer:    model.LayerForRelationType(re.Type),
			Weight:   weight,
			Metadata: meta,
		})
	}
	return relations, nil
}

// EntityFileFrom creates an EntityFile from a model.Entity and its outgoing relations.
func EntityFileFrom(e model.Entity, relations []model.Relation) (EntityFile, error) {
	var meta map[string]any
	if len(e.Metadata) > 0 {
		if err := json.Unmarshal(e.Metadata, &meta); err != nil {
			return EntityFile{}, err
		}
	}

	entries := make([]RelationEntry, 0, len(relations))
	for _, r := range relations {
		re := RelationEntry{
			To:   r.ToID,
			Type: r.Type,
		}
		if r.Weight != 1.0 {
			re.Weight = r.Weight
		}
		if len(r.Metadata) > 0 {
			var rm map[string]any
			if err := json.Unmarshal(r.Metadata, &rm); err != nil {
				return EntityFile{}, err
			}
			re.Metadata = rm
		}
		entries = append(entries, re)
	}

	return EntityFile{
		Schema:      1,
		ID:          e.ID,
		Type:        e.Type,
		Title:       e.Title,
		Description: e.Description,
		Status:      e.Status,
		Metadata:    meta,
		Relations:   entries,
	}, nil
}

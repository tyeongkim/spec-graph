// Package store reads the pre-migration SQLite database. The live write path is
// TOML files plus the derived index; this package exists only so `spec-graph
// migrate` can drain an old graph.db into that path.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// LegacyReader reads entities and relations from a pre-migration database.
type LegacyReader struct {
	db *sql.DB
}

// NewLegacyReader returns a reader over an already-open pre-migration database.
func NewLegacyReader(db *sql.DB) *LegacyReader {
	return &LegacyReader{db: db}
}

// Entities returns every entity in the database, ordered by ID.
func (r *LegacyReader) Entities() ([]model.Entity, error) {
	rows, err := r.db.Query(
		`SELECT id, type, title, description, status, metadata, layer, created_at, updated_at
		 FROM entities ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}
	defer rows.Close()

	entities := make([]model.Entity, 0)
	for rows.Next() {
		var e model.Entity
		var meta string
		var desc sql.NullString
		if err := rows.Scan(&e.ID, &e.Type, &e.Title, &desc, &e.Status, &meta, &e.Layer, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		e.Description = desc.String
		e.Metadata = json.RawMessage(meta)
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entities: %w", err)
	}
	return entities, nil
}

// Relations returns every relation in the database, ordered by ID.
func (r *LegacyReader) Relations() ([]model.Relation, error) {
	rows, err := r.db.Query(
		`SELECT id, from_id, to_id, type, weight, metadata, layer, created_at
		 FROM relations ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	relations := make([]model.Relation, 0)
	for rows.Next() {
		var rel model.Relation
		var meta string
		if err := rows.Scan(&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.Weight, &meta, &rel.Layer, &rel.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		rel.Metadata = []byte(meta)
		relations = append(relations, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}
	return relations, nil
}

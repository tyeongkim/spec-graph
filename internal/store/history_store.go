package store

import (
	"database/sql"
	"fmt"

	"github.com/taeyeong/spec-graph/internal/model"
)

// HistoryStore manages entity and relation history persistence.
// Record methods accept *sql.Tx for atomic use with changeset creation.
// Query methods use *sql.DB directly.
type HistoryStore struct {
	db *sql.DB
}

// NewHistoryStore creates a new HistoryStore.
func NewHistoryStore(db *sql.DB) *HistoryStore {
	return &HistoryStore{db: db}
}

// RecordEntityChange inserts an entity history entry within the given transaction.
func (s *HistoryStore) RecordEntityChange(tx *sql.Tx, entry model.EntityHistoryEntry) error {
	_, err := tx.Exec(
		"INSERT INTO entity_history (changeset_id, entity_id, action, before_json, after_json) VALUES (?, ?, ?, ?, ?)",
		entry.ChangesetID, entry.EntityID, string(entry.Action), entry.BeforeJSON, entry.AfterJSON,
	)
	if err != nil {
		return fmt.Errorf("record entity change: %w", err)
	}
	return nil
}

// RecordRelationChange inserts a relation history entry within the given transaction.
func (s *HistoryStore) RecordRelationChange(tx *sql.Tx, entry model.RelationHistoryEntry) error {
	_, err := tx.Exec(
		"INSERT INTO relation_history (changeset_id, relation_key, action, before_json, after_json) VALUES (?, ?, ?, ?, ?)",
		entry.ChangesetID, entry.RelationKey, string(entry.Action), entry.BeforeJSON, entry.AfterJSON,
	)
	if err != nil {
		return fmt.Errorf("record relation change: %w", err)
	}
	return nil
}

// GetEntityHistory returns all history entries for the given entity, ordered by created_at DESC.
func (s *HistoryStore) GetEntityHistory(entityID string) ([]model.EntityHistoryEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, changeset_id, entity_id, action, before_json, after_json, created_at FROM entity_history WHERE entity_id = ? ORDER BY id ASC",
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query entity history: %w", err)
	}
	defer rows.Close()

	entries := make([]model.EntityHistoryEntry, 0)
	for rows.Next() {
		var e model.EntityHistoryEntry
		var beforeJSON, afterJSON sql.NullString
		if err := rows.Scan(&e.ID, &e.ChangesetID, &e.EntityID, &e.Action, &beforeJSON, &afterJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entity history: %w", err)
		}
		if beforeJSON.Valid {
			e.BeforeJSON = &beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = &afterJSON.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity history: %w", err)
	}
	return entries, nil
}

// GetRelationHistory returns all history entries for the given relation key, ordered by created_at DESC.
func (s *HistoryStore) GetRelationHistory(relationKey string) ([]model.RelationHistoryEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, changeset_id, relation_key, action, before_json, after_json, created_at FROM relation_history WHERE relation_key = ? ORDER BY id ASC",
		relationKey,
	)
	if err != nil {
		return nil, fmt.Errorf("query relation history: %w", err)
	}
	defer rows.Close()

	entries := make([]model.RelationHistoryEntry, 0)
	for rows.Next() {
		var e model.RelationHistoryEntry
		var beforeJSON, afterJSON sql.NullString
		if err := rows.Scan(&e.ID, &e.ChangesetID, &e.RelationKey, &e.Action, &beforeJSON, &afterJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation history: %w", err)
		}
		if beforeJSON.Valid {
			e.BeforeJSON = &beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = &afterJSON.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relation history: %w", err)
	}
	return entries, nil
}

// GetChangesetHistory returns all entity and relation history entries for the given changeset.
func (s *HistoryStore) GetChangesetHistory(changesetID string) ([]model.EntityHistoryEntry, []model.RelationHistoryEntry, error) {
	entityEntries, err := s.queryEntityHistoryByChangeset(changesetID)
	if err != nil {
		return nil, nil, err
	}

	relationEntries, err := s.queryRelationHistoryByChangeset(changesetID)
	if err != nil {
		return nil, nil, err
	}

	return entityEntries, relationEntries, nil
}

func (s *HistoryStore) queryEntityHistoryByChangeset(changesetID string) ([]model.EntityHistoryEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, changeset_id, entity_id, action, before_json, after_json, created_at FROM entity_history WHERE changeset_id = ? ORDER BY id ASC",
		changesetID,
	)
	if err != nil {
		return nil, fmt.Errorf("query entity history by changeset: %w", err)
	}
	defer rows.Close()

	entries := make([]model.EntityHistoryEntry, 0)
	for rows.Next() {
		var e model.EntityHistoryEntry
		var beforeJSON, afterJSON sql.NullString
		if err := rows.Scan(&e.ID, &e.ChangesetID, &e.EntityID, &e.Action, &beforeJSON, &afterJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entity history: %w", err)
		}
		if beforeJSON.Valid {
			e.BeforeJSON = &beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = &afterJSON.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity history: %w", err)
	}
	return entries, nil
}

func (s *HistoryStore) queryRelationHistoryByChangeset(changesetID string) ([]model.RelationHistoryEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, changeset_id, relation_key, action, before_json, after_json, created_at FROM relation_history WHERE changeset_id = ? ORDER BY id ASC",
		changesetID,
	)
	if err != nil {
		return nil, fmt.Errorf("query relation history by changeset: %w", err)
	}
	defer rows.Close()

	entries := make([]model.RelationHistoryEntry, 0)
	for rows.Next() {
		var e model.RelationHistoryEntry
		var beforeJSON, afterJSON sql.NullString
		if err := rows.Scan(&e.ID, &e.ChangesetID, &e.RelationKey, &e.Action, &beforeJSON, &afterJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation history: %w", err)
		}
		if beforeJSON.Valid {
			e.BeforeJSON = &beforeJSON.String
		}
		if afterJSON.Valid {
			e.AfterJSON = &afterJSON.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relation history: %w", err)
	}
	return entries, nil
}

package store

import (
	"database/sql"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// ChangesetStore manages changeset persistence. Create and GetNextID accept
// *sql.Tx so they can participate in mutation transactions. Get and List use
// the underlying *sql.DB directly (read-only, outside transactions).
type ChangesetStore struct {
	db *sql.DB
}

// NewChangesetStore returns a ChangesetStore backed by db.
func NewChangesetStore(db *sql.DB) *ChangesetStore {
	return &ChangesetStore{db: db}
}

// GetNextID generates the next sequential CHG-N id within the given transaction.
func (s *ChangesetStore) GetNextID(tx *sql.Tx) (string, error) {
	var next int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, 5) AS INTEGER)), 0) + 1 FROM changesets",
	).Scan(&next)
	if err != nil {
		return "", fmt.Errorf("get next changeset id: %w", err)
	}
	return fmt.Sprintf("CHG-%d", next), nil
}

// Create inserts a changeset within the given transaction. It reads back the
// row via Get (using s.db) to return the full record including created_at.
func (s *ChangesetStore) Create(tx *sql.Tx, cs model.Changeset) (model.Changeset, error) {
	_, err := tx.Exec(
		"INSERT INTO changesets (id, reason, actor, source) VALUES (?, ?, ?, ?)",
		cs.ID, cs.Reason, nilIfEmpty(cs.Actor), nilIfEmpty(cs.Source),
	)
	if err != nil {
		return model.Changeset{}, fmt.Errorf("insert changeset: %w", err)
	}
	return s.getFromTx(tx, cs.ID)
}

// Get reads a changeset by ID. Returns *model.ErrChangesetNotFound when missing.
func (s *ChangesetStore) Get(id string) (model.Changeset, error) {
	var cs model.Changeset
	var actor, source sql.NullString
	err := s.db.QueryRow(
		"SELECT id, reason, actor, source, created_at FROM changesets WHERE id = ?", id,
	).Scan(&cs.ID, &cs.Reason, &actor, &source, &cs.CreatedAt)
	if err == sql.ErrNoRows {
		return model.Changeset{}, &model.ErrChangesetNotFound{ID: id}
	}
	if err != nil {
		return model.Changeset{}, fmt.Errorf("get changeset %q: %w", id, err)
	}
	if actor.Valid {
		cs.Actor = actor.String
	}
	if source.Valid {
		cs.Source = source.String
	}
	return cs, nil
}

func (s *ChangesetStore) getFromTx(tx *sql.Tx, id string) (model.Changeset, error) {
	var cs model.Changeset
	var actor, source sql.NullString
	err := tx.QueryRow(
		"SELECT id, reason, actor, source, created_at FROM changesets WHERE id = ?", id,
	).Scan(&cs.ID, &cs.Reason, &actor, &source, &cs.CreatedAt)
	if err == sql.ErrNoRows {
		return model.Changeset{}, &model.ErrChangesetNotFound{ID: id}
	}
	if err != nil {
		return model.Changeset{}, fmt.Errorf("get changeset %q: %w", id, err)
	}
	if actor.Valid {
		cs.Actor = actor.String
	}
	if source.Valid {
		cs.Source = source.String
	}
	return cs, nil
}

// List returns all changesets ordered by created_at DESC.
func (s *ChangesetStore) List() ([]model.Changeset, error) {
	rows, err := s.db.Query(
		"SELECT id, reason, actor, source, created_at FROM changesets ORDER BY created_at DESC, CAST(SUBSTR(id, 5) AS INTEGER) DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list changesets: %w", err)
	}
	defer rows.Close()

	result := make([]model.Changeset, 0)
	for rows.Next() {
		var cs model.Changeset
		var actor, source sql.NullString
		if err := rows.Scan(&cs.ID, &cs.Reason, &actor, &source, &cs.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan changeset: %w", err)
		}
		if actor.Valid {
			cs.Actor = actor.String
		}
		if source.Valid {
			cs.Source = source.String
		}
		result = append(result, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate changesets: %w", err)
	}
	return result, nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

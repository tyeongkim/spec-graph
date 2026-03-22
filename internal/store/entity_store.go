package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/taeyeong/spec-graph/internal/model"
)

// EntityFilters holds optional filters for listing entities.
type EntityFilters struct {
	Type   *model.EntityType
	Status *model.EntityStatus
}

// UpdateFields holds optional fields for updating an entity.
type UpdateFields struct {
	Title       *string
	Description *string
	Status      *model.EntityStatus
	Metadata    *json.RawMessage
}

// EntityStore manages entity persistence with automatic changeset and history
// recording for every mutation (Create, Update, Delete).
type EntityStore struct {
	db *sql.DB
	cs *ChangesetStore
	hs *HistoryStore
}

// NewEntityStore returns an EntityStore backed by db with changeset and history stores.
func NewEntityStore(db *sql.DB, cs *ChangesetStore, hs *HistoryStore) *EntityStore {
	return &EntityStore{db: db, cs: cs, hs: hs}
}

// Create inserts a new entity inside a transaction, recording a changeset and
// history entry atomically.
func (s *EntityStore) Create(entity model.Entity, reason, actor, source string) (model.Entity, error) {
	if err := model.ValidateEntityID(entity.ID, entity.Type); err != nil {
		return model.Entity{}, &model.ErrInvalidInput{Message: err.Error()}
	}

	if entity.Status == "" {
		entity.Status = model.EntityStatusDraft
	}
	if entity.Metadata == nil {
		entity.Metadata = json.RawMessage(`{}`)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return model.Entity{}, fmt.Errorf("begin tx: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO entities (id, type, title, description, status, metadata) VALUES (?, ?, ?, ?, ?, ?)`,
		entity.ID, entity.Type, entity.Title, entity.Description, entity.Status, string(entity.Metadata),
	)
	if err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "PRIMARY KEY") {
			return model.Entity{}, &model.ErrDuplicateEntity{ID: entity.ID}
		}
		return model.Entity{}, fmt.Errorf("insert entity: %w", err)
	}

	created, err := s.getFromTx(tx, entity.ID)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("read back entity: %w", err)
	}

	csID, err := s.cs.GetNextID(tx)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("get changeset id: %w", err)
	}
	_, err = s.cs.Create(tx, model.Changeset{ID: csID, Reason: reason, Actor: actor, Source: source})
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("create changeset: %w", err)
	}

	afterJSON, err := json.Marshal(created)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("marshal entity: %w", err)
	}
	afterStr := string(afterJSON)
	if err := s.hs.RecordEntityChange(tx, model.EntityHistoryEntry{
		ChangesetID: csID,
		EntityID:    entity.ID,
		Action:      model.ActionCreate,
		AfterJSON:   &afterStr,
	}); err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Entity{}, fmt.Errorf("commit tx: %w", err)
	}

	return created, nil
}

// Get reads an entity by ID. Read-only, no transaction needed.
func (s *EntityStore) Get(id string) (model.Entity, error) {
	var e model.Entity
	var meta string
	var desc sql.NullString

	err := s.db.QueryRow(
		`SELECT id, type, title, description, status, metadata, created_at, updated_at FROM entities WHERE id = ?`,
		id,
	).Scan(&e.ID, &e.Type, &e.Title, &desc, &e.Status, &meta, &e.CreatedAt, &e.UpdatedAt)

	if err == sql.ErrNoRows {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	if err != nil {
		return model.Entity{}, fmt.Errorf("get entity %q: %w", id, err)
	}

	if desc.Valid {
		e.Description = desc.String
	}
	e.Metadata = json.RawMessage(meta)
	return e, nil
}

// List returns entities matching the given filters. Read-only.
func (s *EntityStore) List(filters EntityFilters) ([]model.Entity, int, error) {
	query := `SELECT id, type, title, description, status, metadata, created_at, updated_at FROM entities`
	var conditions []string
	var args []any

	if filters.Type != nil {
		conditions = append(conditions, "type = ?")
		args = append(args, *filters.Type)
	}
	if filters.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *filters.Status)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	entities := make([]model.Entity, 0)
	for rows.Next() {
		var e model.Entity
		var meta string
		var desc sql.NullString

		if err := rows.Scan(&e.ID, &e.Type, &e.Title, &desc, &e.Status, &meta, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan entity: %w", err)
		}
		if desc.Valid {
			e.Description = desc.String
		}
		e.Metadata = json.RawMessage(meta)
		entities = append(entities, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate entities: %w", err)
	}

	return entities, len(entities), nil
}

// Update modifies an entity inside a transaction, recording a changeset and
// history entry with before/after snapshots.
func (s *EntityStore) Update(id string, fields UpdateFields, reason, actor, source string, action model.HistoryAction) (model.Entity, error) {
	var setClauses []string
	var args []any

	if fields.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *fields.Title)
	}
	if fields.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *fields.Description)
	}
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *fields.Status)
	}
	if fields.Metadata != nil {
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, string(*fields.Metadata))
	}

	if len(setClauses) == 0 {
		return s.Get(id)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return model.Entity{}, fmt.Errorf("begin tx: %w", err)
	}

	before, err := s.getFromTx(tx, id)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, err
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE entities SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("update entity %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		tx.Rollback()
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}

	after, err := s.getFromTx(tx, id)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("read back entity: %w", err)
	}

	csID, err := s.cs.GetNextID(tx)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("get changeset id: %w", err)
	}
	_, err = s.cs.Create(tx, model.Changeset{ID: csID, Reason: reason, Actor: actor, Source: source})
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("create changeset: %w", err)
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("marshal before: %w", err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("marshal after: %w", err)
	}
	beforeStr := string(beforeJSON)
	afterStr := string(afterJSON)
	if err := s.hs.RecordEntityChange(tx, model.EntityHistoryEntry{
		ChangesetID: csID,
		EntityID:    id,
		Action:      action,
		BeforeJSON:  &beforeStr,
		AfterJSON:   &afterStr,
	}); err != nil {
		tx.Rollback()
		return model.Entity{}, fmt.Errorf("record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Entity{}, fmt.Errorf("commit tx: %w", err)
	}

	return after, nil
}

// Delete removes an entity inside a transaction, recording a changeset and
// history entry with the before snapshot.
func (s *EntityStore) Delete(id string, reason, actor, source string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM entities WHERE id = ?)`, id).Scan(&exists)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("check entity existence: %w", err)
	}
	if !exists {
		tx.Rollback()
		return &model.ErrEntityNotFound{ID: id}
	}

	var relCount int
	err = tx.QueryRow(
		`SELECT COUNT(*) FROM relations WHERE from_id = ? OR to_id = ?`, id, id,
	).Scan(&relCount)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("check relations: %w", err)
	}
	if relCount > 0 {
		tx.Rollback()
		return &model.ErrInvalidInput{Message: fmt.Sprintf("entity %q has %d existing relation(s)", id, relCount)}
	}

	before, err := s.getFromTx(tx, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("read entity before delete: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM entities WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete entity %q: %w", id, err)
	}

	csID, err := s.cs.GetNextID(tx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("get changeset id: %w", err)
	}
	_, err = s.cs.Create(tx, model.Changeset{ID: csID, Reason: reason, Actor: actor, Source: source})
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("create changeset: %w", err)
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("marshal before: %w", err)
	}
	beforeStr := string(beforeJSON)
	if err := s.hs.RecordEntityChange(tx, model.EntityHistoryEntry{
		ChangesetID: csID,
		EntityID:    id,
		Action:      model.ActionDelete,
		BeforeJSON:  &beforeStr,
	}); err != nil {
		tx.Rollback()
		return fmt.Errorf("record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// getFromTx reads an entity by ID within a transaction.
func (s *EntityStore) getFromTx(tx *sql.Tx, id string) (model.Entity, error) {
	var e model.Entity
	var meta string
	var desc sql.NullString

	err := tx.QueryRow(
		`SELECT id, type, title, description, status, metadata, created_at, updated_at FROM entities WHERE id = ?`, id,
	).Scan(&e.ID, &e.Type, &e.Title, &desc, &e.Status, &meta, &e.CreatedAt, &e.UpdatedAt)

	if err == sql.ErrNoRows {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	if err != nil {
		return model.Entity{}, fmt.Errorf("get entity %q from tx: %w", id, err)
	}

	if desc.Valid {
		e.Description = desc.String
	}
	e.Metadata = json.RawMessage(meta)
	return e, nil
}

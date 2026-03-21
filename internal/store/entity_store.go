package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/taeyeong/spec-graph/internal/model"
)

type EntityFilters struct {
	Type   *model.EntityType
	Status *model.EntityStatus
}

type UpdateFields struct {
	Title       *string
	Description *string
	Status      *model.EntityStatus
	Metadata    *json.RawMessage
}

type EntityStore struct {
	db *sql.DB
}

func NewEntityStore(db *sql.DB) *EntityStore {
	return &EntityStore{db: db}
}

func (s *EntityStore) Create(entity model.Entity) (model.Entity, error) {
	if err := model.ValidateEntityID(entity.ID, entity.Type); err != nil {
		return model.Entity{}, &model.ErrInvalidInput{Message: err.Error()}
	}

	if entity.Status == "" {
		entity.Status = model.EntityStatusDraft
	}
	if entity.Metadata == nil {
		entity.Metadata = json.RawMessage(`{}`)
	}

	_, err := s.db.Exec(
		`INSERT INTO entities (id, type, title, description, status, metadata) VALUES (?, ?, ?, ?, ?, ?)`,
		entity.ID, entity.Type, entity.Title, entity.Description, entity.Status, string(entity.Metadata),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "PRIMARY KEY") {
			return model.Entity{}, &model.ErrDuplicateEntity{ID: entity.ID}
		}
		return model.Entity{}, fmt.Errorf("insert entity: %w", err)
	}

	return s.Get(entity.ID)
}

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

func (s *EntityStore) Update(id string, fields UpdateFields) (model.Entity, error) {
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

	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE entities SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return model.Entity{}, fmt.Errorf("update entity %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return model.Entity{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}

	return s.Get(id)
}

func (s *EntityStore) Delete(id string) error {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM entities WHERE id = ?)`, id).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check entity existence: %w", err)
	}
	if !exists {
		return &model.ErrEntityNotFound{ID: id}
	}

	var relCount int
	err = s.db.QueryRow(
		`SELECT COUNT(*) FROM relations WHERE from_id = ? OR to_id = ?`, id, id,
	).Scan(&relCount)
	if err != nil {
		return fmt.Errorf("check relations: %w", err)
	}
	if relCount > 0 {
		return &model.ErrInvalidInput{Message: fmt.Sprintf("entity %q has %d existing relation(s)", id, relCount)}
	}

	_, err = s.db.Exec(`DELETE FROM entities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete entity %q: %w", id, err)
	}

	return nil
}

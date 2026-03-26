package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/taeyeong/spec-graph/internal/model"
)

// RelationFilters holds optional filters for listing relations.
type RelationFilters struct {
	FromID *string
	ToID   *string
	Type   *model.RelationType
	Layer  *model.Layer
}

// RelationStore manages relation persistence with edge matrix validation.
type RelationStore struct {
	db *sql.DB
	cs *ChangesetStore
	hs *HistoryStore
}

// NewRelationStore creates a new RelationStore.
func NewRelationStore(db *sql.DB, cs *ChangesetStore, hs *HistoryStore) *RelationStore {
	return &RelationStore{db: db, cs: cs, hs: hs}
}

// Create validates and inserts a relation atomically with changeset and history recording.
// Validation order: from exists → to exists → self-loop → edge matrix → duplicate.
func (s *RelationStore) Create(rel model.Relation, reason, actor, source string) (model.Relation, error) {
	// Validate BEFORE starting transaction (read-only checks).
	fromType, err := s.getEntityType(rel.FromID)
	if err != nil {
		return model.Relation{}, err
	}

	toType, err := s.getEntityType(rel.ToID)
	if err != nil {
		return model.Relation{}, err
	}

	if rel.FromID == rel.ToID {
		return model.Relation{}, &model.ErrSelfLoop{ID: rel.FromID}
	}

	if !model.IsEdgeAllowed(rel.Type, fromType, toType, nil) {
		return model.Relation{}, &model.ErrInvalidEdge{
			FromType:     fromType,
			ToType:       toType,
			RelationType: rel.Type,
		}
	}

	if rel.Weight == 0 {
		rel.Weight = 1.0
	}
	if len(rel.Metadata) == 0 {
		rel.Metadata = []byte("{}")
	}
	rel.Layer = model.LayerForRelationType(rel.Type)

	// Begin transaction for the mutation.
	tx, err := s.db.Begin()
	if err != nil {
		return model.Relation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Create changeset.
	csID, err := s.cs.GetNextID(tx)
	if err != nil {
		return model.Relation{}, fmt.Errorf("get changeset id: %w", err)
	}
	changeset := model.Changeset{ID: csID, Reason: reason, Actor: actor, Source: source}
	if _, err := s.cs.Create(tx, changeset); err != nil {
		return model.Relation{}, fmt.Errorf("create changeset: %w", err)
	}

	// INSERT relation.
	result, err := tx.Exec(
		`INSERT INTO relations (from_id, to_id, type, weight, metadata, layer) VALUES (?, ?, ?, ?, ?, ?)`,
		rel.FromID, rel.ToID, string(rel.Type), rel.Weight, string(rel.Metadata), string(rel.Layer),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return model.Relation{}, &model.ErrDuplicateRelation{
				FromID:       rel.FromID,
				ToID:         rel.ToID,
				RelationType: rel.Type,
			}
		}
		return model.Relation{}, fmt.Errorf("insert relation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Relation{}, fmt.Errorf("last insert id: %w", err)
	}

	// Read back relation within tx.
	var out model.Relation
	var metadata string
	err = tx.QueryRow(
		`SELECT id, from_id, to_id, type, weight, metadata, layer, created_at FROM relations WHERE id = ?`, id,
	).Scan(&out.ID, &out.FromID, &out.ToID, &out.Type, &out.Weight, &metadata, &out.Layer, &out.CreatedAt)
	if err != nil {
		return model.Relation{}, fmt.Errorf("read back relation: %w", err)
	}
	out.Metadata = []byte(metadata)

	// Record history.
	afterJSON, err := json.Marshal(out)
	if err != nil {
		return model.Relation{}, fmt.Errorf("marshal relation: %w", err)
	}
	afterStr := string(afterJSON)
	relationKey := fmt.Sprintf("%s:%s:%s", out.FromID, out.ToID, out.Type)

	if err := s.hs.RecordRelationChange(tx, model.RelationHistoryEntry{
		ChangesetID: csID,
		RelationKey: relationKey,
		Action:      model.ActionCreate,
		AfterJSON:   &afterStr,
	}); err != nil {
		return model.Relation{}, fmt.Errorf("record relation history: %w", err)
	}

	// Commit.
	if err := tx.Commit(); err != nil {
		return model.Relation{}, fmt.Errorf("commit tx: %w", err)
	}

	return out, nil
}

// List returns relations matching the given filters.
// Returns an empty slice (not nil) when no results found.
func (s *RelationStore) List(filters RelationFilters) ([]model.Relation, int, error) {
	query := `SELECT id, from_id, to_id, type, weight, metadata, layer, created_at FROM relations`
	var conditions []string
	var args []any

	if filters.FromID != nil {
		conditions = append(conditions, "from_id = ?")
		args = append(args, *filters.FromID)
	}
	if filters.ToID != nil {
		conditions = append(conditions, "to_id = ?")
		args = append(args, *filters.ToID)
	}
	if filters.Type != nil {
		conditions = append(conditions, "type = ?")
		args = append(args, string(*filters.Type))
	}
	if filters.Layer != nil {
		conditions = append(conditions, "layer = ?")
		args = append(args, string(*filters.Layer))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	rels := make([]model.Relation, 0)
	for rows.Next() {
		var r model.Relation
		var metadata string
		if err := rows.Scan(&r.ID, &r.FromID, &r.ToID, &r.Type, &r.Weight, &metadata, &r.Layer, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan relation: %w", err)
		}
		r.Metadata = []byte(metadata)
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate relations: %w", err)
	}

	return rels, len(rels), nil
}

// GetByEntity returns all relations where the given entity is either from_id or to_id.
// Returns an empty slice (not nil) when no relations found.
func (s *RelationStore) GetByEntity(entityID string) ([]model.Relation, error) {
	rows, err := s.db.Query(
		`SELECT id, from_id, to_id, type, weight, metadata, layer, created_at FROM relations WHERE from_id = ? OR to_id = ? ORDER BY id`,
		entityID, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query relations by entity: %w", err)
	}
	defer rows.Close()

	rels := make([]model.Relation, 0)
	for rows.Next() {
		var r model.Relation
		var metadata string
		if err := rows.Scan(&r.ID, &r.FromID, &r.ToID, &r.Type, &r.Weight, &metadata, &r.Layer, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		r.Metadata = []byte(metadata)
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}

	return rels, nil
}

// Delete removes a relation by (from_id, to_id, type) atomically with changeset and history recording.
// Returns ErrRelationNotFound if no matching relation exists.
func (s *RelationStore) Delete(fromID, toID string, relType model.RelationType, reason, actor, source string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Read current relation state for before_json.
	var rel model.Relation
	var metadata string
	err = tx.QueryRow(
		`SELECT id, from_id, to_id, type, weight, metadata, layer, created_at FROM relations WHERE from_id = ? AND to_id = ? AND type = ?`,
		fromID, toID, string(relType),
	).Scan(&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.Weight, &metadata, &rel.Layer, &rel.CreatedAt)
	if err == sql.ErrNoRows {
		return &model.ErrRelationNotFound{ID: 0}
	}
	if err != nil {
		return fmt.Errorf("read relation for delete: %w", err)
	}
	rel.Metadata = []byte(metadata)

	// Create changeset.
	csID, err := s.cs.GetNextID(tx)
	if err != nil {
		return fmt.Errorf("get changeset id: %w", err)
	}
	changeset := model.Changeset{ID: csID, Reason: reason, Actor: actor, Source: source}
	if _, err := s.cs.Create(tx, changeset); err != nil {
		return fmt.Errorf("create changeset: %w", err)
	}

	// DELETE relation.
	if _, err := tx.Exec(
		`DELETE FROM relations WHERE from_id = ? AND to_id = ? AND type = ?`,
		fromID, toID, string(relType),
	); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}

	// Record history.
	beforeJSON, err := json.Marshal(rel)
	if err != nil {
		return fmt.Errorf("marshal relation: %w", err)
	}
	beforeStr := string(beforeJSON)
	relationKey := fmt.Sprintf("%s:%s:%s", fromID, toID, relType)

	if err := s.hs.RecordRelationChange(tx, model.RelationHistoryEntry{
		ChangesetID: csID,
		RelationKey: relationKey,
		Action:      model.ActionDelete,
		BeforeJSON:  &beforeStr,
	}); err != nil {
		return fmt.Errorf("record relation history: %w", err)
	}

	// Commit.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// HasRelations checks whether an entity participates in any relation (as from or to).
func (s *RelationStore) HasRelations(entityID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM relations WHERE from_id = ? OR to_id = ?)`,
		entityID, entityID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check relations: %w", err)
	}
	return exists, nil
}

// getEntityType returns the entity type for the given ID, or ErrEntityNotFound.
func (s *RelationStore) getEntityType(id string) (model.EntityType, error) {
	var entityType model.EntityType
	err := s.db.QueryRow(`SELECT type FROM entities WHERE id = ?`, id).Scan(&entityType)
	if err == sql.ErrNoRows {
		return "", &model.ErrEntityNotFound{ID: id}
	}
	if err != nil {
		return "", fmt.Errorf("get entity type %q: %w", id, err)
	}
	return entityType, nil
}

// Package index provides a disposable SQLite query index rebuilt from TOML
// source files. The index lives at .spec-graph/graph.db and is .gitignored.
// It stores only what is needed for queries — git is the audit log.
package index

import (
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

var connectionPragmas = []string{
	"busy_timeout(30000)",
	"foreign_keys(1)",
	"synchronous(NORMAL)",
	"temp_store(2)",
}

// EntityRecord is the data needed to populate the index.
type EntityRecord struct {
	ID          string
	Type        string
	Layer       string
	Status      string
	Title       string
	Description string
	Metadata    string
	FilePath    string
	CreatedAt   string
	UpdatedAt   string
}

// RelationRecord is the data needed to populate the index.
type RelationRecord struct {
	FromID   string
	ToID     string
	Type     string
	Layer    string
	Weight   float64
	Metadata string
}

// EntityFilters for query filtering.
type EntityFilters struct {
	Type   string
	Status string
	Layer  string
}

// RelationFilters for query filtering.
type RelationFilters struct {
	FromID string
	ToID   string
	Type   string
	Layer  string
}

// Index is a disposable SQLite query index rebuilt from TOML source files.
type Index struct {
	db   *sql.DB
	path string
}

// Open opens or creates the index database at the given path.
func Open(path string) (*Index, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, fmt.Errorf("index open %q: %w", path, err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("index apply schema: %w", err)
	}

	return &Index{db: db, path: path}, nil
}

// Close closes the database connection.
func (idx *Index) Close() error {
	if idx.db == nil {
		return nil
	}
	return idx.db.Close()
}

// DB returns the underlying *sql.DB for raw read-only queries (e.g. query sql).
func (idx *Index) DB() *sql.DB {
	return idx.db
}

// Rebuild drops all data and repopulates from the provided entities and
// relations. This is atomic: builds into a temp DB, then renames over the
// existing one. The caller provides already-parsed data from TOML files.
func (idx *Index) Rebuild(entities []EntityRecord, relations []RelationRecord) error {
	tmpPath := idx.path + ".tmp"

	os.Remove(tmpPath)

	tmpDB, err := openDB(tmpPath)
	if err != nil {
		return fmt.Errorf("rebuild open temp db: %w", err)
	}

	if _, err := tmpDB.Exec(schemaSQL); err != nil {
		tmpDB.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild apply schema: %w", err)
	}

	if err := insertEntities(tmpDB, entities); err != nil {
		tmpDB.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild insert entities: %w", err)
	}

	if err := insertRelations(tmpDB, relations); err != nil {
		tmpDB.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild insert relations: %w", err)
	}

	if _, err := tmpDB.Exec(`INSERT INTO entities_fts(id, title) SELECT id, title FROM entities`); err != nil {
		tmpDB.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild populate fts: %w", err)
	}

	if err := tmpDB.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild close temp db: %w", err)
	}

	if err := idx.db.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rebuild close current db: %w", err)
	}

	os.Remove(idx.path + "-wal")
	os.Remove(idx.path + "-shm")

	if err := os.Rename(tmpPath, idx.path); err != nil {
		return fmt.Errorf("rebuild rename: %w", err)
	}

	os.Remove(tmpPath + "-wal")
	os.Remove(tmpPath + "-shm")

	db, err := openDB(idx.path)
	if err != nil {
		return fmt.Errorf("rebuild reopen: %w", err)
	}
	idx.db = db

	return nil
}

// GetEntity returns a single entity by ID, or nil if not found.
func (idx *Index) GetEntity(id string) (*EntityRecord, error) {
	row := idx.db.QueryRow(
		`SELECT id, type, layer, status, title, description, metadata, file_path, created_at, updated_at FROM entities WHERE id = ?`, id,
	)
	var e EntityRecord
	err := row.Scan(&e.ID, &e.Type, &e.Layer, &e.Status, &e.Title, &e.Description, &e.Metadata, &e.FilePath, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %q: %w", id, err)
	}
	return &e, nil
}

// ListEntities returns entities matching the given filters. Empty filter fields
// are ignored.
func (idx *Index) ListEntities(filters EntityFilters) ([]EntityRecord, error) {
	query := `SELECT id, type, layer, status, title, description, metadata, file_path, created_at, updated_at FROM entities`
	var conditions []string
	var args []any

	if filters.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filters.Type)
	}
	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.Layer != "" {
		conditions = append(conditions, "layer = ?")
		args = append(args, filters.Layer)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id"

	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// ListRelations returns relations matching the given filters. Empty filter
// fields are ignored.
func (idx *Index) ListRelations(filters RelationFilters) ([]RelationRecord, error) {
	query := `SELECT from_id, to_id, type, layer, weight, metadata FROM relations`
	var conditions []string
	var args []any

	if filters.FromID != "" {
		conditions = append(conditions, "from_id = ?")
		args = append(args, filters.FromID)
	}
	if filters.ToID != "" {
		conditions = append(conditions, "to_id = ?")
		args = append(args, filters.ToID)
	}
	if filters.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filters.Type)
	}
	if filters.Layer != "" {
		conditions = append(conditions, "layer = ?")
		args = append(args, filters.Layer)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY from_id, to_id"

	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

// GetRelationsByEntity returns all relations where the entity is either the
// source or target.
func (idx *Index) GetRelationsByEntity(entityID string) ([]RelationRecord, error) {
	rows, err := idx.db.Query(
		`SELECT from_id, to_id, type, layer, weight, metadata FROM relations WHERE from_id = ? OR to_id = ? ORDER BY from_id, to_id`,
		entityID, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("get relations by entity %q: %w", entityID, err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

// SearchEntities performs a full-text search on entity titles.
func (idx *Index) SearchEntities(query string) ([]EntityRecord, error) {
	rows, err := idx.db.Query(
		`SELECT e.id, e.type, e.layer, e.status, e.title, e.description, e.metadata, e.file_path, e.created_at, e.updated_at
		 FROM entities_fts f
		 JOIN entities e ON e.id = f.id
		 WHERE entities_fts MATCH ?
		 ORDER BY rank`,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("search entities %q: %w", query, err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// GetMeta returns the value for a metadata key, or empty string if not found.
func (idx *Index) GetMeta(key string) (string, error) {
	var value string
	err := idx.db.QueryRow(`SELECT value FROM _meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get meta %q: %w", key, err)
	}
	return value, nil
}

// SetMeta sets a metadata key-value pair (upsert).
func (idx *Index) SetMeta(key, value string) error {
	_, err := idx.db.Exec(
		`INSERT INTO _meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}

func buildDSN(path string) string {
	q := url.Values{}
	for _, p := range connectionPragmas {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")

	if path == ":memory:" {
		return ":memory:?" + q.Encode()
	}
	return "file:" + path + "?" + q.Encode()
}

func openDB(path string) (*sql.DB, error) {
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	return db, nil
}

func insertEntities(db *sql.DB, entities []EntityRecord) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO entities(id, type, layer, status, title, description, metadata, file_path, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range entities {
		if _, err := stmt.Exec(e.ID, e.Type, e.Layer, e.Status, e.Title, e.Description, e.Metadata, e.FilePath, e.CreatedAt, e.UpdatedAt); err != nil {
			return fmt.Errorf("insert entity %q: %w", e.ID, err)
		}
	}

	return tx.Commit()
}

func insertRelations(db *sql.DB, relations []RelationRecord) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO relations(from_id, to_id, type, layer, weight, metadata) VALUES(?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range relations {
		w := r.Weight
		if w == 0 {
			w = 1.0
		}
		meta := r.Metadata
		if meta == "" {
			meta = "{}"
		}
		if _, err := stmt.Exec(r.FromID, r.ToID, r.Type, r.Layer, w, meta); err != nil {
			return fmt.Errorf("insert relation %q->%q: %w", r.FromID, r.ToID, err)
		}
	}

	return tx.Commit()
}

func scanEntities(rows *sql.Rows) ([]EntityRecord, error) {
	var result []EntityRecord
	for rows.Next() {
		var e EntityRecord
		if err := rows.Scan(&e.ID, &e.Type, &e.Layer, &e.Status, &e.Title, &e.Description, &e.Metadata, &e.FilePath, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return result, nil
}

func scanRelations(rows *sql.Rows) ([]RelationRecord, error) {
	var result []RelationRecord
	for rows.Next() {
		var r RelationRecord
		if err := rows.Scan(&r.FromID, &r.ToID, &r.Type, &r.Layer, &r.Weight, &r.Metadata); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return result, nil
}

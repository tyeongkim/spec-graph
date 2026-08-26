package store

import (
	"database/sql"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/db"
)

func setupLegacyDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.OpenMemoryDB()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func TestLegacyReaderReturnsEmptySlices(t *testing.T) {
	r := NewLegacyReader(setupLegacyDB(t))

	entities, err := r.Entities()
	if err != nil {
		t.Fatalf("Entities: %v", err)
	}
	if len(entities) != 0 {
		t.Errorf("want no entities, got %#v", entities)
	}

	relations, err := r.Relations()
	if err != nil {
		t.Fatalf("Relations: %v", err)
	}
	if len(relations) != 0 {
		t.Errorf("want no relations, got %#v", relations)
	}
}

func TestLegacyReaderReadsEntitiesAndRelations(t *testing.T) {
	d := setupLegacyDB(t)
	mustExec(t, d,
		`INSERT INTO entities (id, type, title, description, status, metadata, layer)
		 VALUES ('REQ-002', 'requirement', 'Second', NULL, 'draft', '{}', 'arch'),
		        ('REQ-001', 'requirement', 'First', 'desc', 'active', '{"k":1}', 'arch')`)
	mustExec(t, d,
		`INSERT INTO relations (from_id, to_id, type, weight, metadata, layer)
		 VALUES ('REQ-001', 'REQ-002', 'depends_on', 1.0, '{}', 'arch')`)

	r := NewLegacyReader(d)

	entities, err := r.Entities()
	if err != nil {
		t.Fatalf("Entities: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("want 2 entities, got %d", len(entities))
	}
	if entities[0].ID != "REQ-001" || entities[1].ID != "REQ-002" {
		t.Errorf("want ID order [REQ-001 REQ-002], got [%s %s]", entities[0].ID, entities[1].ID)
	}
	if entities[0].Description != "desc" {
		t.Errorf("want description %q, got %q", "desc", entities[0].Description)
	}
	if string(entities[0].Metadata) != `{"k":1}` {
		t.Errorf("want metadata %q, got %q", `{"k":1}`, entities[0].Metadata)
	}
	if entities[1].Description != "" {
		t.Errorf("want NULL description read as empty string, got %q", entities[1].Description)
	}

	relations, err := r.Relations()
	if err != nil {
		t.Fatalf("Relations: %v", err)
	}
	if len(relations) != 1 {
		t.Fatalf("want 1 relation, got %d", len(relations))
	}
	if relations[0].FromID != "REQ-001" || relations[0].ToID != "REQ-002" {
		t.Errorf("unexpected relation endpoints: %+v", relations[0])
	}
}

func TestLegacyReaderPropagatesQueryError(t *testing.T) {
	d := setupLegacyDB(t)
	mustExec(t, d, `DROP TABLE relations`)
	mustExec(t, d, `DROP TABLE entities`)

	r := NewLegacyReader(d)
	if _, err := r.Entities(); err == nil {
		t.Error("want error when entities table is missing")
	}
	if _, err := r.Relations(); err == nil {
		t.Error("want error when relations table is missing")
	}
}

func mustExec(t *testing.T, d *sql.DB, query string) {
	t.Helper()
	if _, err := d.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

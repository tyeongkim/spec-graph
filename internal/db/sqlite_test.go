package db

import (
	"database/sql"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	return db
}

func TestPragmaWALMode(t *testing.T) {
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB failed: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	// In-memory databases return "memory" for journal_mode, not "wal".
	// WAL is only meaningful for file-based databases.
	// We verify the PRAGMA was executed by checking OpenDB sets it on file DBs.
	// For memory DB, just confirm we can query the pragma without error.
	if journalMode != "memory" && journalMode != "wal" {
		t.Errorf("PRAGMA journal_mode = %q; want %q or %q", journalMode, "wal", "memory")
	}
}

func TestPragmaWALModeFileDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("PRAGMA journal_mode = %q; want %q", journalMode, "wal")
	}
}

func TestPragmaForeignKeys(t *testing.T) {
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB failed: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("PRAGMA foreign_keys = %d; want 1", fk)
	}
}

func TestMigrateCreatesEntitiesTable(t *testing.T) {
	db := setupTestDB(t)

	rows, err := db.Query("PRAGMA table_info(entities)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(entities): %v", err)
	}
	defer rows.Close()

	expectedCols := map[string]bool{
		"id": false, "type": false, "title": false, "description": false,
		"status": false, "metadata": false, "created_at": false, "updated_at": false,
	}

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		if _, ok := expectedCols[name]; ok {
			expectedCols[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	for col, found := range expectedCols {
		if !found {
			t.Errorf("entities table missing column %q", col)
		}
	}
}

func TestMigrateCreatesRelationsTable(t *testing.T) {
	db := setupTestDB(t)

	rows, err := db.Query("PRAGMA table_info(relations)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(relations): %v", err)
	}
	defer rows.Close()

	expectedCols := map[string]bool{
		"id": false, "from_id": false, "to_id": false, "type": false,
		"weight": false, "metadata": false, "created_at": false,
	}

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		if _, ok := expectedCols[name]; ok {
			expectedCols[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	for col, found := range expectedCols {
		if !found {
			t.Errorf("relations table missing column %q", col)
		}
	}
}

func TestCheckConstraintRejectsInvalidEntityType(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('X-001', 'bogus', 'test', 'draft')`)
	if err == nil {
		t.Error("expected CHECK constraint error for invalid entity type 'bogus', got nil")
	}
}

func TestCheckConstraintRejectsInvalidEntityStatus(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-001', 'requirement', 'test', 'unknown')`)
	if err == nil {
		t.Error("expected CHECK constraint error for invalid entity status 'unknown', got nil")
	}
}

func TestCheckConstraintRejectsInvalidRelationType(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-001', 'requirement', 'req1', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity: %v", err)
	}
	_, err = db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-002', 'requirement', 'req2', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity: %v", err)
	}

	_, err = db.Exec(`INSERT INTO relations (from_id, to_id, type) VALUES ('REQ-001', 'REQ-002', 'invalid_type')`)
	if err == nil {
		t.Error("expected CHECK constraint error for invalid relation type 'invalid_type', got nil")
	}
}

func TestUniqueConstraintOnRelations(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-001', 'requirement', 'req1', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-002', 'requirement', 'req2', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity 2: %v", err)
	}

	_, err = db.Exec(`INSERT INTO relations (from_id, to_id, type) VALUES ('REQ-001', 'REQ-002', 'depends_on')`)
	if err != nil {
		t.Fatalf("first relation insert: %v", err)
	}

	_, err = db.Exec(`INSERT INTO relations (from_id, to_id, type) VALUES ('REQ-001', 'REQ-002', 'depends_on')`)
	if err == nil {
		t.Error("expected UNIQUE constraint error for duplicate (from_id, to_id, type), got nil")
	}
}

func TestForeignKeyEnforcedOnRelations(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO relations (from_id, to_id, type) VALUES ('NONEXIST-001', 'NONEXIST-002', 'depends_on')`)
	if err == nil {
		t.Error("expected foreign key error for non-existent from_id/to_id, got nil")
	}
}

func TestValidEntityInsert(t *testing.T) {
	db := setupTestDB(t)

	validTypes := []struct {
		id     string
		etype  string
		status string
	}{
		{"REQ-001", "requirement", "draft"},
		{"DEC-001", "decision", "active"},
		{"PHS-001", "phase", "deprecated"},
		{"API-001", "interface", "resolved"},
		{"STT-001", "state", "deleted"},
		{"TST-001", "test", "draft"},
		{"XCT-001", "crosscut", "active"},
		{"QST-001", "question", "draft"},
		{"ASM-001", "assumption", "draft"},
		{"ACT-001", "criterion", "draft"},
		{"RSK-001", "risk", "draft"},
	}

	for _, tt := range validTypes {
		t.Run(tt.etype+"/"+tt.status, func(t *testing.T) {
			_, err := db.Exec(
				`INSERT INTO entities (id, type, title, status) VALUES (?, ?, ?, ?)`,
				tt.id, tt.etype, "test "+tt.etype, tt.status,
			)
			if err != nil {
				t.Errorf("valid insert (%s, %s) failed: %v", tt.etype, tt.status, err)
			}
		})
	}
}

func TestValidRelationInsert(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-010', 'requirement', 'r', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity: %v", err)
	}
	_, err = db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('DEC-010', 'decision', 'd', 'draft')`)
	if err != nil {
		t.Fatalf("insert entity: %v", err)
	}

	validRelTypes := []string{
		"implements", "verifies", "depends_on", "constrained_by",
		"triggers", "answers", "assumes",
		"has_criterion", "mitigates", "supersedes", "conflicts_with", "references",
		"belongs_to", "precedes", "blocks", "covers", "delivers",
	}

	for _, rt := range validRelTypes {
		t.Run(rt, func(t *testing.T) {
			_, err := db.Exec(
				`INSERT INTO relations (from_id, to_id, type) VALUES (?, ?, ?)`,
				"REQ-010", "DEC-010", rt,
			)
			if err != nil {
				t.Errorf("valid relation insert (%s) failed: %v", rt, err)
			}
		})
	}
}

package db

import (
	"database/sql"
	"testing"
)

func setupMigrateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrateAppliesInOrder(t *testing.T) {
	db := setupMigrateTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	if len(versions) == 0 {
		t.Fatal("expected at least one migration version, got none")
	}

	if versions[0] != "001_init" {
		t.Errorf("first version = %q; want %q", versions[0], "001_init")
	}

	for i := 1; i < len(versions); i++ {
		if versions[i] <= versions[i-1] {
			t.Errorf("versions not in order: %q <= %q", versions[i], versions[i-1])
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := setupMigrateTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate failed: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = '001_init'").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("001_init appears %d times; want 1", count)
	}
}

func TestMigrateTracksVersions(t *testing.T) {
	db := setupMigrateTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	var version, appliedAt string
	err := db.QueryRow("SELECT version, applied_at FROM schema_migrations WHERE version = '001_init'").Scan(&version, &appliedAt)
	if err != nil {
		t.Fatalf("query 001_init: %v", err)
	}

	if version != "001_init" {
		t.Errorf("version = %q; want %q", version, "001_init")
	}
	if appliedAt == "" {
		t.Error("applied_at is empty")
	}
}

func TestMigrateCreatesTrackingTable(t *testing.T) {
	db := setupMigrateTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&name)
	if err != nil {
		t.Fatalf("schema_migrations table not found: %v", err)
	}
}

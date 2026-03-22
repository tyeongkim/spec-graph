package db

import (
	"testing"
)

func TestMigrateCreatesHistoryTables(t *testing.T) {
	db := setupTestDB(t)

	tables := []string{"changesets", "entity_history", "relation_history"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var name string
			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
			).Scan(&name)
			if err != nil {
				t.Errorf("table %q not found: %v", table, err)
			}
		})
	}
}

func TestChangesetInsert(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(
		`INSERT INTO changesets (id, reason, actor, source) VALUES (?, ?, ?, ?)`,
		"cs-001", "test reason", "user@example.com", "cli",
	)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}
}

func TestEntityHistoryInsertValid(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO changesets (id, reason) VALUES ('cs-001', 'test')`)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}

	after := `{"id":"REQ-001"}`
	_, err = db.Exec(
		`INSERT INTO entity_history (changeset_id, entity_id, action, after_json) VALUES (?, ?, ?, ?)`,
		"cs-001", "REQ-001", "create", after,
	)
	if err != nil {
		t.Fatalf("insert entity_history: %v", err)
	}
}

func TestEntityHistoryForeignKeyEnforced(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(
		`INSERT INTO entity_history (changeset_id, entity_id, action) VALUES (?, ?, ?)`,
		"nonexistent-cs", "REQ-001", "create",
	)
	if err == nil {
		t.Error("expected FK error for nonexistent changeset_id, got nil")
	}
}

func TestEntityHistoryCheckConstraintAction(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO changesets (id, reason) VALUES ('cs-001', 'test')`)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO entity_history (changeset_id, entity_id, action) VALUES (?, ?, ?)`,
		"cs-001", "REQ-001", "invalid_action",
	)
	if err == nil {
		t.Error("expected CHECK constraint error for invalid action, got nil")
	}
}

func TestEntityHistoryValidActions(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO changesets (id, reason) VALUES ('cs-001', 'test')`)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}

	actions := []string{"create", "update", "deprecate", "delete"}
	for i, action := range actions {
		t.Run(action, func(t *testing.T) {
			entityID := "REQ-00" + string(rune('1'+i))
			_, err := db.Exec(
				`INSERT INTO entity_history (changeset_id, entity_id, action) VALUES (?, ?, ?)`,
				"cs-001", entityID, action,
			)
			if err != nil {
				t.Errorf("valid action %q failed: %v", action, err)
			}
		})
	}
}

func TestRelationHistoryInsertValid(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO changesets (id, reason) VALUES ('cs-001', 'test')`)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}

	before := `{"from_id":"REQ-001","to_id":"DEC-001","type":"depends_on"}`
	_, err = db.Exec(
		`INSERT INTO relation_history (changeset_id, relation_key, action, before_json) VALUES (?, ?, ?, ?)`,
		"cs-001", "REQ-001:depends_on:DEC-001", "delete", before,
	)
	if err != nil {
		t.Fatalf("insert relation_history: %v", err)
	}
}

func TestRelationHistoryCheckConstraintAction(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO changesets (id, reason) VALUES ('cs-001', 'test')`)
	if err != nil {
		t.Fatalf("insert changeset: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO relation_history (changeset_id, relation_key, action) VALUES (?, ?, ?)`,
		"cs-001", "REQ-001:depends_on:DEC-001", "update",
	)
	if err == nil {
		t.Error("expected CHECK constraint error for invalid relation action 'update', got nil")
	}
}

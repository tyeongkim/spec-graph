package index

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLegacyIndexInvalidatesFingerprintForCompletionAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE entities (
 id TEXT PRIMARY KEY, type TEXT NOT NULL, layer TEXT NOT NULL,
 status TEXT NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
 metadata TEXT NOT NULL DEFAULT '{}', file_path TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE _meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO _meta VALUES ('toml_fingerprint', 'already-synced');
INSERT INTO entities(id, type, layer, status, title, file_path)
VALUES ('PHS-001', 'phase', 'exec', 'resolved', 'Accepted', 'phase.toml');`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	fingerprint, err := idx.GetMeta("toml_fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint != "" {
		t.Fatalf("legacy fingerprint retained, preventing audit backfill: %q", fingerprint)
	}
	record, err := idx.GetEntity("PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Title != "Accepted" {
		t.Fatalf("migration lost legacy entity: %+v", record)
	}
	if err := idx.Rebuild([]EntityRecord{{
		ID: "PHS-001", Type: "phase", Layer: "exec", Status: "resolved", Title: "Accepted",
		CompletionForced: true, CompletionReason: "Accepted risk",
	}}, nil); err != nil {
		t.Fatal(err)
	}
	record, err = idx.GetEntity("PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	if !record.CompletionForced || record.CompletionReason != "Accepted risk" {
		t.Fatalf("rebuilt index lost audit: %+v", record)
	}
	found, err := idx.SearchEntities("Accepted")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || !found[0].CompletionForced || found[0].CompletionReason != "Accepted risk" {
		t.Fatalf("search lost audit: %+v", found)
	}
}

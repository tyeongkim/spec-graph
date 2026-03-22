package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

func TestHistoryEntityAfterCreate(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "History Test")
	if r.exitCode != 0 {
		t.Fatalf("entity add failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "history", "entity", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("history entity failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityHistoryResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.EntityID != "REQ-001" {
		t.Errorf("entity_id = %q; want REQ-001", resp.EntityID)
	}
	if resp.Count != 1 {
		t.Errorf("count = %d; want 1", resp.Count)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("len(entries) = %d; want 1", len(resp.Entries))
	}
	if resp.Entries[0].Action != "create" {
		t.Errorf("action = %q; want create", resp.Entries[0].Action)
	}
	if resp.Entries[0].EntityID != "REQ-001" {
		t.Errorf("entry entity_id = %q; want REQ-001", resp.Entries[0].EntityID)
	}
}

func TestHistoryEntityMultipleEntries(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Original")
	runCLI(t, dir, "--db", dbFile, "entity", "update", "REQ-001",
		"--title", "Updated")

	r := runCLI(t, dir, "--db", dbFile, "history", "entity", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("history entity failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityHistoryResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d; want 2", resp.Count)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("len(entries) = %d; want 2", len(resp.Entries))
	}
}

func TestHistoryEntityNoHistory(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "history", "entity", "REQ-999")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0 for empty history, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityHistoryResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.EntityID != "REQ-999" {
		t.Errorf("entity_id = %q; want REQ-999", resp.EntityID)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d; want 0", resp.Count)
	}
	if resp.Entries == nil {
		t.Error("entries should be non-nil empty slice")
	}
}

func TestHistoryChangeset(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "CS Test")

	r := runCLI(t, dir, "--db", dbFile, "history", "changeset", "CHG-1")
	if r.exitCode != 0 {
		t.Fatalf("history changeset failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ChangesetResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Changeset.ID != "CHG-1" {
		t.Errorf("changeset.id = %q; want CHG-1", resp.Changeset.ID)
	}
	if len(resp.EntityEntries) != 1 {
		t.Errorf("len(entity_entries) = %d; want 1", len(resp.EntityEntries))
	}
	if resp.EntityEntries[0].Action != "create" {
		t.Errorf("entity entry action = %q; want create", resp.EntityEntries[0].Action)
	}
}

func TestHistoryChangesetNotFound(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "history", "changeset", "CHG-999")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "CHANGESET_NOT_FOUND" {
		t.Errorf("code = %q; want CHANGESET_NOT_FOUND", errResp.Error.Code)
	}
}

func TestHistoryRelationNoHistory(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "history", "relation", "REQ-001:DEC-001:depends_on")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0 for empty relation history, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationHistoryResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.RelationKey != "REQ-001:DEC-001:depends_on" {
		t.Errorf("relation_key = %q; want REQ-001:DEC-001:depends_on", resp.RelationKey)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d; want 0", resp.Count)
	}
}

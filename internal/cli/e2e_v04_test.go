package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func TestE2E_V04_HistoryTracking(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	t.Run("entity_create", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "requirement", "--id", "REQ-001", "--title", "Auth Required",
			"--reason", "initial setup", "--actor", "dev-1", "--source", "spec.md")
		if r.exitCode != 0 {
			t.Fatalf("entity add failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
	})

	t.Run("entity_update", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "update", "REQ-001",
			"--title", "Authentication Required",
			"--reason", "clarify title", "--actor", "dev-1")
		if r.exitCode != 0 {
			t.Fatalf("entity update failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
	})

	t.Run("entity_deprecate", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "deprecate", "REQ-001",
			"--reason", "replaced by REQ-002", "--actor", "dev-2")
		if r.exitCode != 0 {
			t.Fatalf("entity deprecate failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
	})

	t.Run("history_entity_three_entries", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "history", "entity", "REQ-001")
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
		if resp.Count != 3 {
			t.Fatalf("count = %d; want 3", resp.Count)
		}
		if len(resp.Entries) != 3 {
			t.Fatalf("len(entries) = %d; want 3", len(resp.Entries))
		}

		// SQLite has second-level timestamp precision; lookup by action, not index.
		actionMap := map[string]jsoncontract.EntityHistoryEntry{}
		for _, e := range resp.Entries {
			actionMap[e.Action] = e
		}

		createEntry, ok := actionMap["create"]
		if !ok {
			t.Fatal("missing create entry")
		}
		if createEntry.Before != nil {
			t.Error("create entry: before should be nil")
		}
		if createEntry.After == nil {
			t.Error("create entry: after should be non-nil")
		}

		updateEntry, ok := actionMap["update"]
		if !ok {
			t.Fatal("missing update entry")
		}
		if updateEntry.Before == nil {
			t.Error("update entry: before should be non-nil")
		}
		if updateEntry.After == nil {
			t.Error("update entry: after should be non-nil")
		}

		if _, ok := actionMap["deprecate"]; !ok {
			t.Fatal("missing deprecate entry")
		}

		csIDs := map[string]bool{}
		for _, e := range resp.Entries {
			csIDs[e.ChangesetID] = true
		}
		if len(csIDs) != 3 {
			t.Errorf("expected 3 distinct changeset IDs, got %d", len(csIDs))
		}
	})

	t.Run("history_changeset_detail", func(t *testing.T) {
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
		if resp.Changeset.Reason != "initial setup" {
			t.Errorf("changeset.reason = %q; want 'initial setup'", resp.Changeset.Reason)
		}
		if resp.Changeset.Actor != "dev-1" {
			t.Errorf("changeset.actor = %q; want dev-1", resp.Changeset.Actor)
		}
		if resp.Changeset.Source != "spec.md" {
			t.Errorf("changeset.source = %q; want spec.md", resp.Changeset.Source)
		}
		if len(resp.EntityEntries) != 1 {
			t.Fatalf("len(entity_entries) = %d; want 1", len(resp.EntityEntries))
		}
		if resp.EntityEntries[0].Action != "create" {
			t.Errorf("entity entry action = %q; want create", resp.EntityEntries[0].Action)
		}
	})

	t.Run("relation_create_and_delete", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "interface", "--id", "API-001", "--title", "Auth API")
		if r.exitCode != 0 {
			t.Fatalf("entity add API-001 failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		r = runCLI(t, dir, "--db", dbFile, "relation", "add",
			"--from", "API-001", "--to", "REQ-001", "--type", "implements",
			"--reason", "API implements auth", "--actor", "dev-1")
		if r.exitCode != 0 {
			t.Fatalf("relation add failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		r = runCLI(t, dir, "--db", dbFile, "relation", "delete",
			"--from", "API-001", "--to", "REQ-001", "--type", "implements",
			"--reason", "removing link", "--actor", "dev-2")
		if r.exitCode != 0 {
			t.Fatalf("relation delete failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
	})

	t.Run("history_relation_two_entries", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "history", "relation", "API-001:REQ-001:implements")
		if r.exitCode != 0 {
			t.Fatalf("history relation failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.RelationHistoryResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}
		if resp.RelationKey != "API-001:REQ-001:implements" {
			t.Errorf("relation_key = %q; want API-001:REQ-001:implements", resp.RelationKey)
		}
		if resp.Count != 2 {
			t.Fatalf("count = %d; want 2", resp.Count)
		}
		if len(resp.Entries) != 2 {
			t.Fatalf("len(entries) = %d; want 2", len(resp.Entries))
		}

		actionMap := map[string]jsoncontract.RelationHistoryEntry{}
		for _, e := range resp.Entries {
			actionMap[e.Action] = e
		}

		createEntry, ok := actionMap["create"]
		if !ok {
			t.Fatal("missing create entry for relation")
		}
		if createEntry.After == nil {
			t.Error("relation create: after should be non-nil")
		}

		deleteEntry, ok := actionMap["delete"]
		if !ok {
			t.Fatal("missing delete entry for relation")
		}
		if deleteEntry.Before == nil {
			t.Error("relation delete: before should be non-nil")
		}
		if deleteEntry.After != nil {
			t.Error("relation delete: after should be nil")
		}
	})
}

func TestE2E_V04_BootstrapPipeline(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	scanDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fixture := `# REQ-010 User Authentication
All users must authenticate before accessing the system.

# DEC-010 Use OAuth2
We decided to use OAuth2 for authentication.

REQ-010 depends on DEC-010
`
	if err := os.WriteFile(filepath.Join(scanDir, "spec.md"), []byte(fixture), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var scanStdout string

	t.Run("bootstrap_scan", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", scanDir)
		if r.exitCode != 0 {
			t.Fatalf("bootstrap scan failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		scanStdout = r.stdout

		var resp jsoncontract.BootstrapScanResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}

		entityMap := map[string]jsoncontract.BootstrapEntityCandidate{}
		for _, e := range resp.Entities {
			entityMap[e.ID] = e
		}

		req, ok := entityMap["REQ-010"]
		if !ok {
			t.Fatal("REQ-010 not found in scan results")
		}
		if req.Type != "requirement" {
			t.Errorf("REQ-010 type = %q; want requirement", req.Type)
		}
		if req.Confidence <= 0 {
			t.Errorf("REQ-010 confidence = %f; want > 0", req.Confidence)
		}

		dec, ok := entityMap["DEC-010"]
		if !ok {
			t.Fatal("DEC-010 not found in scan results")
		}
		if dec.Type != "decision" {
			t.Errorf("DEC-010 type = %q; want decision", dec.Type)
		}
	})

	candidatesFile := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(candidatesFile, []byte(scanStdout), 0644); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	t.Run("bootstrap_import_review", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "bootstrap", "import",
			"--input", candidatesFile, "--mode", "review")
		if r.exitCode != 0 {
			t.Fatalf("import review failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.BootstrapScanResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}
		if len(resp.Entities) == 0 {
			t.Fatal("expected entities in review output")
		}
	})

	t.Run("review_did_not_write", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "list")
		if r.exitCode != 0 {
			t.Fatalf("entity list failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityListResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Count != 0 {
			t.Errorf("review should not create entities; count = %d", resp.Count)
		}
	})

	t.Run("bootstrap_import_apply", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "bootstrap", "import",
			"--input", candidatesFile, "--mode", "apply")
		if r.exitCode != 0 {
			t.Fatalf("import apply failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.BootstrapImportResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}

		createdSet := map[string]bool{}
		for _, id := range resp.Created {
			createdSet[id] = true
		}
		if !createdSet["REQ-010"] {
			t.Errorf("REQ-010 not in created list: %v", resp.Created)
		}
		if !createdSet["DEC-010"] {
			t.Errorf("DEC-010 not in created list: %v", resp.Created)
		}
	})

	t.Run("entity_get_after_apply", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-010")
		if r.exitCode != 0 {
			t.Fatalf("entity get REQ-010 failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Entity.ID != "REQ-010" {
			t.Errorf("entity id = %q; want REQ-010", resp.Entity.ID)
		}
		if resp.Entity.Type != "requirement" {
			t.Errorf("entity type = %q; want requirement", resp.Entity.Type)
		}
		if resp.Entity.Title != "User Authentication" {
			t.Errorf("entity title = %q; want 'User Authentication'", resp.Entity.Title)
		}
	})

	t.Run("entity_list_count", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "list")
		if r.exitCode != 0 {
			t.Fatalf("entity list failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityListResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("entity count = %d; want 2", resp.Count)
		}
	})

	t.Run("history_bootstrap_entity", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "history", "entity", "REQ-010")
		if r.exitCode != 0 {
			t.Fatalf("history entity failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.EntityHistoryResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}
		if resp.Count != 1 {
			t.Fatalf("count = %d; want 1", resp.Count)
		}
		if len(resp.Entries) != 1 {
			t.Fatalf("len(entries) = %d; want 1", len(resp.Entries))
		}
		if resp.Entries[0].Action != "create" {
			t.Errorf("action = %q; want create", resp.Entries[0].Action)
		}

		csID := resp.Entries[0].ChangesetID
		r = runCLI(t, dir, "--db", dbFile, "history", "changeset", csID)
		if r.exitCode != 0 {
			t.Fatalf("history changeset %s failed: exit=%d stderr=%s", csID, r.exitCode, r.stderr)
		}

		var csResp jsoncontract.ChangesetResponse
		if err := json.Unmarshal([]byte(r.stdout), &csResp); err != nil {
			t.Fatalf("unmarshal changeset: %v\nraw: %s", err, r.stdout)
		}
		if csResp.Changeset.Reason != "bootstrap import" {
			t.Errorf("changeset reason = %q; want 'bootstrap import'", csResp.Changeset.Reason)
		}
		if csResp.Changeset.Source != "bootstrap" {
			t.Errorf("changeset source = %q; want 'bootstrap'", csResp.Changeset.Source)
		}
	})
}

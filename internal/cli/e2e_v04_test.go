package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

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
}

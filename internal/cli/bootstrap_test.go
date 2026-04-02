package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func TestBootstrapScanDirectory(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	scanDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scanDir, "reqs.md"), []byte("# REQ-001 Auth requirement\nAll APIs need auth.\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", scanDir)
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.BootstrapScanResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if len(resp.Entities) == 0 {
		t.Fatal("expected at least one entity candidate")
	}

	found := false
	for _, e := range resp.Entities {
		if e.ID == "REQ-001" {
			found = true
			if e.Type != "requirement" {
				t.Errorf("type = %q; want requirement", e.Type)
			}
			if e.Title != "Auth requirement" {
				t.Errorf("title = %q; want 'Auth requirement'", e.Title)
			}
		}
	}
	if !found {
		t.Error("REQ-001 not found in scan results")
	}

	if resp.Relations == nil {
		t.Error("relations should be non-nil (empty slice)")
	}
}

func TestBootstrapScanFile(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	fixture := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(fixture, []byte("# DEC-001 Use REST\nWe chose REST over gRPC.\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", fixture)
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.BootstrapScanResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if len(resp.Entities) != 1 {
		t.Fatalf("len(entities) = %d; want 1", len(resp.Entities))
	}
	if resp.Entities[0].ID != "DEC-001" {
		t.Errorf("id = %q; want DEC-001", resp.Entities[0].ID)
	}
}

func TestBootstrapScanMissingInput(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestBootstrapImportReview(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	fixture := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(fixture, []byte("# REQ-001 Auth requirement\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", fixture)
	if r.exitCode != 0 {
		t.Fatalf("scan failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	candidatesFile := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(candidatesFile, []byte(r.stdout), 0644); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	r = runCLI(t, dir, "--db", dbFile, "bootstrap", "import", "--input", candidatesFile, "--mode", "review")
	if r.exitCode != 0 {
		t.Fatalf("import review failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.BootstrapScanResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if len(resp.Entities) == 0 {
		t.Fatal("expected at least one entity in review output")
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("entity list failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var listResp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Count != 0 {
		t.Errorf("review should not create entities; count = %d", listResp.Count)
	}
}

func TestBootstrapImportApply(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	fixture := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(fixture, []byte("# REQ-001 Auth requirement\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", fixture)
	if r.exitCode != 0 {
		t.Fatalf("scan failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	candidatesFile := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(candidatesFile, []byte(r.stdout), 0644); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	r = runCLI(t, dir, "--db", dbFile, "bootstrap", "import", "--input", candidatesFile, "--mode", "apply")
	if r.exitCode != 0 {
		t.Fatalf("import apply failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.BootstrapImportResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	foundCreated := false
	for _, id := range resp.Created {
		if id == "REQ-001" {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Errorf("REQ-001 not in created list: %v", resp.Created)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("entity get failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var entityResp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &entityResp); err != nil {
		t.Fatalf("unmarshal entity: %v", err)
	}
	if entityResp.Entity.ID != "REQ-001" {
		t.Errorf("entity id = %q; want REQ-001", entityResp.Entity.ID)
	}
}

func TestBootstrapImportDefaultMode(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	fixture := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(fixture, []byte("# REQ-001 Auth requirement\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "scan", "--input", fixture)
	if r.exitCode != 0 {
		t.Fatalf("scan failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	candidatesFile := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(candidatesFile, []byte(r.stdout), 0644); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	r = runCLI(t, dir, "--db", dbFile, "bootstrap", "import", "--input", candidatesFile)
	if r.exitCode != 0 {
		t.Fatalf("import without --mode failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.BootstrapScanResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if len(resp.Entities) == 0 {
		t.Fatal("expected at least one entity in default (review) output")
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("entity list failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var listResp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Count != 0 {
		t.Errorf("default mode (review) should not create entities; count = %d", listResp.Count)
	}
}

func TestBootstrapImportInvalidMode(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	candidatesFile := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(candidatesFile, []byte(`{"entities":[],"relations":[]}`), 0644); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	r := runCLI(t, dir, "--db", dbFile, "bootstrap", "import", "--input", candidatesFile, "--mode", "invalid")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

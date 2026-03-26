package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
)

func setupRelationEntities(t *testing.T, dbFile string) {
	t.Helper()
	dir := t.TempDir()

	entities := []struct {
		typ, id, title string
	}{
		{"interface", "API-005", "Payment API"},
		{"requirement", "REQ-001", "Auth Required"},
		{"test", "TST-012", "Auth Test"},
		{"criterion", "ACT-001", "Login Criterion"},
		{"decision", "DEC-001", "Use JWT"},
		{"question", "QST-001", "Which provider?"},
		{"phase", "PHS-001", "Phase 1"},
		{"state", "STT-001", "Authenticated"},
		{"crosscut", "XCT-001", "Audit Logging"},
		{"assumption", "ASM-001", "Single tenant"},
		{"risk", "RSK-001", "Token leak"},
	}

	for _, e := range entities {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", e.typ, "--id", e.id, "--title", e.title)
		if r.exitCode != 0 {
			t.Fatalf("setup entity %s failed: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}
}

func TestRelationAddImplements(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Relation.FromID != "API-005" {
		t.Errorf("from_id = %q; want API-005", resp.Relation.FromID)
	}
	if resp.Relation.ToID != "REQ-001" {
		t.Errorf("to_id = %q; want REQ-001", resp.Relation.ToID)
	}
	if resp.Relation.Type != model.RelationImplements {
		t.Errorf("type = %q; want implements", resp.Relation.Type)
	}
	if resp.Relation.Weight != 1.0 {
		t.Errorf("weight = %f; want 1.0", resp.Relation.Weight)
	}
}

func TestRelationAddVariousTypes(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	cases := []struct {
		name, from, to, relType string
	}{
		{"verifies", "TST-012", "REQ-001", "verifies"},
		{"covers", "PHS-001", "REQ-001", "covers"},
		{"delivers", "PHS-001", "API-005", "delivers"},
		{"answers", "DEC-001", "QST-001", "answers"},
		{"triggers", "API-005", "STT-001", "triggers"},
		{"assumes", "REQ-001", "ASM-001", "assumes"},
		{"has_criterion", "REQ-001", "ACT-001", "has_criterion"},
		{"mitigates", "DEC-001", "RSK-001", "mitigates"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runCLI(t, dir, "--db", dbFile, "relation", "add",
				"--from", tc.from, "--to", tc.to, "--type", tc.relType)
			if r.exitCode != 0 {
				t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
			}

			var resp jsoncontract.RelationResponse
			if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if string(resp.Relation.Type) != tc.relType {
				t.Errorf("type = %q; want %q", resp.Relation.Type, tc.relType)
			}
		})
	}
}

func TestRelationAddSelfLoop(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "SELF_LOOP" {
		t.Errorf("code = %q; want SELF_LOOP", errResp.Error.Code)
	}
}

func TestRelationAddInvalidEdge(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "API-005", "--type", "implements")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "INVALID_EDGE" {
		t.Errorf("code = %q; want INVALID_EDGE", errResp.Error.Code)
	}
}

func TestRelationAddEntityNotFound(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-999", "--type", "implements")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code = %q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

func TestRelationAddDuplicate(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("first add failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "DUPLICATE_RELATION" {
		t.Errorf("code = %q; want DUPLICATE_RELATION", errResp.Error.Code)
	}
}

func TestRelationAddInvalidType(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "bogus_type")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for invalid relation type")
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestRelationAddMissingFlags(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required flags")
	}
}

func TestRelationListByFrom(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "STT-001", "--type", "triggers")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "TST-012", "--to", "REQ-001", "--type", "verifies")

	r := runCLI(t, dir, "--db", dbFile, "relation", "list", "--from", "API-005")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d; want 2", resp.Count)
	}
}

func TestRelationListByTo(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "TST-012", "--to", "REQ-001", "--type", "verifies")

	r := runCLI(t, dir, "--db", dbFile, "relation", "list", "--to", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d; want 2", resp.Count)
	}
}

func TestRelationListByType(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "TST-012", "--to", "REQ-001", "--type", "verifies")

	r := runCLI(t, dir, "--db", dbFile, "relation", "list", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("count = %d; want 1", resp.Count)
	}
}

func TestRelationListAll(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "TST-012", "--to", "REQ-001", "--type", "verifies")

	r := runCLI(t, dir, "--db", dbFile, "relation", "list")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d; want 2", resp.Count)
	}
}

func TestRelationListEmptyResult(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "list", "--from", "REQ-999")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d; want 0", resp.Count)
	}
	if resp.Relations == nil {
		t.Error("relations should be non-nil empty slice")
	}
}

func TestRelationDelete(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")

	r := runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.DeleteResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Deleted == "" {
		t.Error("expected non-empty deleted field")
	}
}

func TestRelationDeleteNotFound(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "RELATION_NOT_FOUND" {
		t.Errorf("code = %q; want RELATION_NOT_FOUND", errResp.Error.Code)
	}
}

func TestRelationDeleteMissingFlags(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "delete")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required flags")
	}
}

func TestRelationFullLifecycle(t *testing.T) {
	dbFile := initTestProject(t)
	setupRelationEntities(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("add failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "list")
	if r.exitCode != 0 {
		t.Fatalf("list failed: %s", r.stderr)
	}
	var listResp jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Count != 1 {
		t.Errorf("list count = %d; want 1", listResp.Count)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("delete failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "list")
	if r.exitCode != 0 {
		t.Fatalf("list after delete failed: %s", r.stderr)
	}
	if err := json.Unmarshal([]byte(r.stdout), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Count != 0 {
		t.Errorf("list count after delete = %d; want 0", listResp.Count)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "API-005", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1 for double delete, got %d", r.exitCode)
	}
}

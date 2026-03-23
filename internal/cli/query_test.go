package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

func TestQueryScopeHappyPath(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	// Create phase.
	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	if r.exitCode != 0 {
		t.Fatalf("add phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	// Create entities.
	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Requirement 1")
	if r.exitCode != 0 {
		t.Fatalf("add req: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "interface", "--id", "API-001", "--title", "Interface 1")
	if r.exitCode != 0 {
		t.Fatalf("add api: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	// Create planned_in relation: REQ-001 -> PHS-001.
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "PHS-001", "--type", "planned_in")
	if r.exitCode != 0 {
		t.Fatalf("add planned_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	// Create delivered_in relation: API-001 -> PHS-001.
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "PHS-001", "--type", "delivered_in")
	if r.exitCode != 0 {
		t.Fatalf("add delivered_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	// Query scope.
	r = runCLI(t, dir, "--db", dbFile, "query", "scope", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("query scope: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryScopeResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.PhaseID != "PHS-001" {
		t.Errorf("phase_id = %q; want PHS-001", resp.PhaseID)
	}
	if len(resp.Entities) != 2 {
		t.Errorf("len(entities) = %d; want 2", len(resp.Entities))
	}
	if len(resp.Relations) != 2 {
		t.Errorf("len(relations) = %d; want 2", len(resp.Relations))
	}
	if resp.Summary.Total != 2 {
		t.Errorf("summary.total = %d; want 2", resp.Summary.Total)
	}

	// Verify by_type counts.
	if resp.Summary.ByType["requirement"] != 1 {
		t.Errorf("by_type[requirement] = %d; want 1", resp.Summary.ByType["requirement"])
	}
	if resp.Summary.ByType["interface"] != 1 {
		t.Errorf("by_type[interface] = %d; want 1", resp.Summary.ByType["interface"])
	}
}

func TestQueryScopeNotFound(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "query", "scope", "PHS-999")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code = %q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

func TestQueryScopeEmptyPhase(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	// Create phase with no linked entities.
	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Empty Phase")
	if r.exitCode != 0 {
		t.Fatalf("add phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "query", "scope", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("query scope: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryScopeResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.PhaseID != "PHS-001" {
		t.Errorf("phase_id = %q; want PHS-001", resp.PhaseID)
	}
	if len(resp.Entities) != 0 {
		t.Errorf("len(entities) = %d; want 0", len(resp.Entities))
	}
	if resp.Entities == nil {
		t.Error("entities should be non-nil empty slice")
	}
	if resp.Summary.Total != 0 {
		t.Errorf("summary.total = %d; want 0", resp.Summary.Total)
	}
}

func TestQueryUnresolvedMixed(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "question", "--id", "QST-001", "--title", "Open question")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "assumption", "--id", "ASM-001", "--title", "Active assumption", "--status", "active")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-001", "--title", "Draft risk")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "question", "--id", "QST-002", "--title", "Resolved question", "--status", "resolved")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Not unresolved type")

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryUnresolvedResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if len(resp.Entities) != 3 {
		t.Fatalf("len(entities) = %d; want 3", len(resp.Entities))
	}
	if resp.Summary.Total != 3 {
		t.Errorf("summary.total = %d; want 3", resp.Summary.Total)
	}

	// Sorted by type then ID: assumption < question < risk
	if resp.Entities[0].ID != "ASM-001" {
		t.Errorf("entities[0].id = %q; want ASM-001", resp.Entities[0].ID)
	}
	if resp.Entities[1].ID != "QST-001" {
		t.Errorf("entities[1].id = %q; want QST-001", resp.Entities[1].ID)
	}
	if resp.Entities[2].ID != "RSK-001" {
		t.Errorf("entities[2].id = %q; want RSK-001", resp.Entities[2].ID)
	}
}

func TestQueryUnresolvedTypeFilter(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "question", "--id", "QST-001", "--title", "Q1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "assumption", "--id", "ASM-001", "--title", "A1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-001", "--title", "R1")

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved", "--type", "question")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryUnresolvedResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if len(resp.Entities) != 1 {
		t.Fatalf("len(entities) = %d; want 1", len(resp.Entities))
	}
	if resp.Entities[0].ID != "QST-001" {
		t.Errorf("entities[0].id = %q; want QST-001", resp.Entities[0].ID)
	}
	if resp.Summary.ByType["question"] != 1 {
		t.Errorf("by_type[question] = %d; want 1", resp.Summary.ByType["question"])
	}
}

func TestQueryUnresolvedEmpty(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryUnresolvedResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Entities == nil {
		t.Error("entities should be non-nil empty slice")
	}
	if len(resp.Entities) != 0 {
		t.Errorf("len(entities) = %d; want 0", len(resp.Entities))
	}
	if resp.Summary.Total != 0 {
		t.Errorf("summary.total = %d; want 0", resp.Summary.Total)
	}
}

func TestQueryUnresolvedInvalidType(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved", "--type", "invalid")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestQueryPathExists(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "interface", "--id", "API-001", "--title", "API 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "REQ-001", "--type", "implements")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "PHS-001", "--type", "delivered_in")

	r := runCLI(t, dir, "--db", dbFile, "query", "path", "REQ-001", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryPathResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if !resp.Found {
		t.Fatal("expected found=true")
	}
	if resp.From != "REQ-001" {
		t.Errorf("from = %q; want REQ-001", resp.From)
	}
	if resp.To != "PHS-001" {
		t.Errorf("to = %q; want PHS-001", resp.To)
	}
	if resp.Length != 2 {
		t.Errorf("length = %d; want 2", resp.Length)
	}
	if len(resp.Path) != 3 {
		t.Fatalf("len(path) = %d; want 3", len(resp.Path))
	}
	if resp.Path[0].EntityID != "REQ-001" {
		t.Errorf("path[0].entity_id = %q; want REQ-001", resp.Path[0].EntityID)
	}
	if resp.Path[0].Relation != "" {
		t.Errorf("path[0].relation = %q; want empty (origin)", resp.Path[0].Relation)
	}
	if resp.Path[1].EntityID != "API-001" {
		t.Errorf("path[1].entity_id = %q; want API-001", resp.Path[1].EntityID)
	}
	if resp.Path[2].EntityID != "PHS-001" {
		t.Errorf("path[2].entity_id = %q; want PHS-001", resp.Path[2].EntityID)
	}
}

func TestQueryPathNoPath(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Dec 1")

	r := runCLI(t, dir, "--db", dbFile, "query", "path", "REQ-001", "DEC-001")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryPathResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Found {
		t.Error("expected found=false")
	}
	if len(resp.Path) != 0 {
		t.Errorf("len(path) = %d; want 0", len(resp.Path))
	}
	if resp.Length != 0 {
		t.Errorf("length = %d; want 0", resp.Length)
	}
}

func TestQueryPathSameEntity(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req 1")

	r := runCLI(t, dir, "--db", dbFile, "query", "path", "REQ-001", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryPathResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if !resp.Found {
		t.Fatal("expected found=true")
	}
	if len(resp.Path) != 1 {
		t.Fatalf("len(path) = %d; want 1", len(resp.Path))
	}
	if resp.Path[0].EntityID != "REQ-001" {
		t.Errorf("path[0].entity_id = %q; want REQ-001", resp.Path[0].EntityID)
	}
	if resp.Length != 0 {
		t.Errorf("length = %d; want 0", resp.Length)
	}
}

func TestQueryPathNonexistentEntity(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req 1")

	r := runCLI(t, dir, "--db", dbFile, "query", "path", "REQ-001", "NONEXISTENT-001")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code = %q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

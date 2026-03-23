package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

// seedOrphanGraph creates a graph with one orphaned entity (no relations).
func seedOrphanGraph(t *testing.T, dir, dbFile string) {
	t.Helper()
	entities := []struct{ typ, id, title string }{
		{"requirement", "REQ-001", "Connected Req"},
		{"interface", "API-001", "Connected API"},
		{"requirement", "REQ-002", "Orphaned Req"},
	}
	for _, e := range entities {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", e.typ, "--id", e.id, "--title", e.title)
		if r.exitCode != 0 {
			t.Fatalf("seed entity %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}
	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("seed relation: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
}

// seedCleanGraph creates a minimal graph that passes all 4 checks.
// REQ-001 (active): has implements (from API-001) + has_criterion (to ACT-001)
// ACT-001 (active): has verifies (from TST-001)
// API-001 (active): connected via implements
// TST-001 (active): connected via verifies
func seedCleanGraph(t *testing.T, dir, dbFile string) {
	t.Helper()
	entities := []struct{ typ, id, title, status string }{
		{"requirement", "REQ-001", "Auth Requirement", "active"},
		{"interface", "API-001", "Auth API", "active"},
		{"criterion", "ACT-001", "Login returns JWT", "active"},
		{"test", "TST-001", "Auth Test", "active"},
	}
	for _, e := range entities {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", e.typ, "--id", e.id, "--title", e.title, "--status", e.status)
		if r.exitCode != 0 {
			t.Fatalf("seed entity %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}
	relations := []struct{ from, to, relType string }{
		{"API-001", "REQ-001", "implements"},
		{"REQ-001", "ACT-001", "has_criterion"},
		{"TST-001", "ACT-001", "verifies"},
		{"TST-001", "REQ-001", "verifies"},
	}
	for _, rel := range relations {
		r := runCLI(t, dir, "--db", dbFile, "relation", "add",
			"--from", rel.from, "--to", rel.to, "--type", rel.relType)
		if r.exitCode != 0 {
			t.Fatalf("seed relation %s->%s: exit=%d stderr=%s",
				rel.from, rel.to, r.exitCode, r.stderr)
		}
	}
}

func TestValidateAllChecks(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedOrphanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	if len(resp.Issues) == 0 {
		t.Error("expected at least one issue")
	}

	foundOrphan := false
	for _, issue := range resp.Issues {
		if issue.Check == "orphans" && issue.Entity == "REQ-002" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Error("expected orphan issue for REQ-002")
	}
}

func TestValidateCheckOrphans(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedOrphanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "orphans")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	for _, issue := range resp.Issues {
		if issue.Check != "orphans" {
			t.Errorf("expected only orphan issues, got check=%q", issue.Check)
		}
	}
}

func TestValidateCheckCoverage(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Uncovered Req", "--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("seed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Some Decision")
	if r.exitCode != 0 {
		t.Fatalf("seed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "DEC-001", "--type", "depends_on")
	if r.exitCode != 0 {
		t.Fatalf("seed relation: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "validate", "--check", "coverage")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	foundCoverage := false
	for _, issue := range resp.Issues {
		if issue.Check == "coverage" && issue.Entity == "REQ-001" {
			foundCoverage = true
		}
	}
	if !foundCoverage {
		t.Error("expected coverage issue for REQ-001")
	}
}

func TestValidateCheckInvalidEdges(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "invalid_edges")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Valid {
		t.Errorf("expected valid=true, got issues: %+v", resp.Issues)
	}
}

func TestValidateCheckSupersededRefs(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "superseded_refs")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Valid {
		t.Errorf("expected valid=true, got issues: %+v", resp.Issues)
	}
}

func TestValidateCleanGraph(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Valid {
		t.Errorf("expected valid=true, got issues: %+v", resp.Issues)
	}
	if len(resp.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %+v", len(resp.Issues), resp.Issues)
	}
	if resp.Summary.TotalIssues != 0 {
		t.Errorf("expected total_issues=0, got %d", resp.Summary.TotalIssues)
	}
}

func TestValidateWithIssues(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedOrphanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	if len(resp.Issues) == 0 {
		t.Error("expected non-empty issues")
	}
	if resp.Summary.TotalIssues != len(resp.Issues) {
		t.Errorf("summary.total_issues=%d; want %d", resp.Summary.TotalIssues, len(resp.Issues))
	}
	for i, issue := range resp.Issues {
		if issue.Check == "" {
			t.Errorf("issue[%d]: empty check", i)
		}
		if issue.Severity == "" {
			t.Errorf("issue[%d]: empty severity", i)
		}
		if issue.Entity == "" {
			t.Errorf("issue[%d]: empty entity", i)
		}
		if issue.Message == "" {
			t.Errorf("issue[%d]: empty message", i)
		}
	}
}

func TestValidateInvalidCheckName(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "nonexistent")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code=%q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestValidatePhaseFlag_InvalidID(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "validate", "--phase", "NONEXISTENT")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code=%q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestValidatePhaseFlag_WrongType(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Some Req")
	if r.exitCode != 0 {
		t.Fatalf("seed entity: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "validate", "--phase", "REQ-001")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code=%q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestValidatePhaseFlag_ValidPhase(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase One")
	if r.exitCode != 0 {
		t.Fatalf("seed phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "PHS-001", "--type", "planned_in")
	if r.exitCode != 0 {
		t.Fatalf("seed planned_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "PHS-001", "--type", "delivered_in")
	if r.exitCode != 0 {
		t.Fatalf("seed delivered_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "validate", "--check", "orphans", "--phase", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Valid {
		t.Errorf("expected valid=true, got issues: %+v", resp.Issues)
	}
}

func seedGateGraph(t *testing.T, dir, dbFile string) {
	t.Helper()
	entities := []struct{ typ, id, title, status string }{
		{"phase", "PHS-001", "Phase One", "active"},
		{"question", "QST-001", "Open Question", "active"},
		{"risk", "RSK-001", "High Risk", "active"},
		{"requirement", "REQ-001", "Auth Req", "active"},
		{"decision", "DEC-001", "Draft Decision", "draft"},
		{"assumption", "ASM-001", "Key Assumption", "active"},
	}
	for _, e := range entities {
		args := []string{"--db", dbFile, "entity", "add",
			"--type", e.typ, "--id", e.id, "--title", e.title}
		if e.status != "" {
			args = append(args, "--status", e.status)
		}
		r := runCLI(t, dir, args...)
		if r.exitCode != 0 {
			t.Fatalf("seed entity %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}
	relations := []struct{ from, to, relType string }{
		{"QST-001", "PHS-001", "planned_in"},
		{"RSK-001", "PHS-001", "planned_in"},
		{"REQ-001", "PHS-001", "planned_in"},
		{"DEC-001", "PHS-001", "planned_in"},
		{"REQ-001", "DEC-001", "depends_on"},
		{"REQ-001", "ASM-001", "assumes"},
	}
	for _, rel := range relations {
		r := runCLI(t, dir, "--db", dbFile, "relation", "add",
			"--from", rel.from, "--to", rel.to, "--type", rel.relType)
		if r.exitCode != 0 {
			t.Fatalf("seed relation %s->%s: exit=%d stderr=%s",
				rel.from, rel.to, r.exitCode, r.stderr)
		}
	}
}

func TestValidateGates_AllViolations(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedGateGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "gates", "--phase", "PHS-001")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	if len(resp.Issues) == 0 {
		t.Error("expected at least one gate issue")
	}
	for _, issue := range resp.Issues {
		if issue.Check != "gates" {
			t.Errorf("expected only gates issues, got check=%q", issue.Check)
		}
	}
}

func TestValidateGates_PhaseScoped(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedGateGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--phase", "PHS-001")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
}

func TestValidateGates_NoPhase(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedGateGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "gates")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	for _, issue := range resp.Issues {
		if issue.Check != "gates" {
			t.Errorf("expected only gates issues, got check=%q", issue.Check)
		}
	}
}

func TestValidateGates_CleanPhase(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-002", "--title", "Clean Phase")
	if r.exitCode != 0 {
		t.Fatalf("seed phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "PHS-002", "--type", "planned_in")
	if r.exitCode != 0 {
		t.Fatalf("seed planned_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "API-001", "--to", "PHS-002", "--type", "delivered_in")
	if r.exitCode != 0 {
		t.Fatalf("seed delivered_in: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "validate", "--check", "gates", "--phase", "PHS-002")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Valid {
		t.Errorf("expected valid=true, got issues: %+v", resp.Issues)
	}
}

func TestValidateMultipleChecks(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedOrphanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--check", "orphans,coverage")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}

	checks := make(map[string]bool)
	for _, issue := range resp.Issues {
		checks[issue.Check] = true
	}
	if !checks["orphans"] {
		t.Error("expected orphan issues from --check orphans,coverage")
	}
}

func TestValidateEntityFlag_FiltersIssues(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedOrphanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "validate", "--entity", "REQ-002")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.ValidateResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	for _, issue := range resp.Issues {
		if issue.Entity != "REQ-002" {
			t.Errorf("expected only issues for REQ-002, got entity=%q", issue.Entity)
		}
	}
	if len(resp.Issues) == 0 {
		t.Error("expected at least one issue for REQ-002")
	}
}

func TestValidateEntityFlag_NonExistent(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "validate", "--entity", "NONEXIST")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code=%q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

func TestValidateEntityFlag_MutualExclusionWithPhase(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	seedCleanGraph(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase One")
	if r.exitCode != 0 {
		t.Fatalf("seed phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "validate", "--entity", "REQ-001", "--phase", "PHS-001")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code=%q; want INVALID_INPUT", errResp.Error.Code)
	}
}

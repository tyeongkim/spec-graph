package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func setupPhaseNextProject(t *testing.T, dbFile string) {
	t.Helper()
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "plan", "--id", "PLN-001", "--title", "Test Plan", "--status", "active")

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1",
		"--metadata", `{"goal":"Foundation","order":1}`)

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-002", "--title", "Phase 2",
		"--metadata", `{"goal":"Core Services","order":2}`)

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-003", "--title", "Phase 3",
		"--metadata", `{"goal":"Features","order":3}`)

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Auth")

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-002", "--title", "CRUD")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "PLN-001", "--type", "belongs_to")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-002", "--to", "PLN-001", "--type", "belongs_to")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-003", "--to", "PLN-001", "--type", "belongs_to")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "PHS-002", "--type", "precedes")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-002", "--to", "PHS-003", "--type", "precedes")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "REQ-001", "--type", "covers")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-002", "--to", "REQ-002", "--type", "covers")
}

func TestPhaseNextSelectsLowestOrder(t *testing.T) {
	dbFile := initTestProject(t)
	setupPhaseNextProject(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "phase", "next")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.PhaseNextResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Phase.ID != "PHS-001" {
		t.Errorf("phase.id = %q; want PHS-001", resp.Phase.ID)
	}
	if resp.Phase.Goal != "Foundation" {
		t.Errorf("phase.goal = %q; want Foundation", resp.Phase.Goal)
	}
	if resp.Phase.Order != 1 {
		t.Errorf("phase.order = %f; want 1", resp.Phase.Order)
	}
	if !resp.Phase.PredecessorsResolved {
		t.Error("predecessors_resolved should be true")
	}
	if resp.Activated {
		t.Error("activated should be false without --activate")
	}
	if resp.Scope.Total != 1 {
		t.Errorf("scope.total = %d; want 1", resp.Scope.Total)
	}
}

func TestPhaseNextActivateFlag(t *testing.T) {
	dbFile := initTestProject(t)
	setupPhaseNextProject(t, dbFile)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "phase", "next", "--activate")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.PhaseNextResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if !resp.Activated {
		t.Error("activated should be true with --activate")
	}
	if resp.Phase.Status != "active" {
		t.Errorf("phase.status = %q; want active", resp.Phase.Status)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("entity get: exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	var entityResp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &entityResp); err != nil {
		t.Fatalf("unmarshal entity: %v", err)
	}
	if entityResp.Entity.Status != model.EntityStatusActive {
		t.Errorf("persisted status = %q; want active", entityResp.Entity.Status)
	}
}

func TestPhaseNextRespectsPrerequisites(t *testing.T) {
	dbFile := initTestProject(t)
	setupPhaseNextProject(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "update", "PHS-001",
		"--status", "resolved", "--force", "--reason", "Advance test fixture")

	r := runCLI(t, dir, "--db", dbFile, "phase", "next")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.PhaseNextResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Phase.ID != "PHS-002" {
		t.Errorf("phase.id = %q; want PHS-002 (PHS-001 resolved, PHS-002 unlocked)", resp.Phase.ID)
	}
}

func TestPhaseNextNoActivePlan(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "plan", "--id", "PLN-001", "--title", "Draft Plan")

	r := runCLI(t, dir, "--db", dbFile, "phase", "next")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit when no active plan")
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal error: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestPhaseNextAllResolved(t *testing.T) {
	dbFile := initTestProject(t)
	setupPhaseNextProject(t, dbFile)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "update", "PHS-001",
		"--status", "resolved", "--force", "--reason", "Resolve test fixture")
	runCLI(t, dir, "--db", dbFile, "entity", "update", "PHS-002",
		"--status", "resolved", "--force", "--reason", "Resolve test fixture")
	runCLI(t, dir, "--db", dbFile, "entity", "update", "PHS-003",
		"--status", "resolved", "--force", "--reason", "Resolve test fixture")

	r := runCLI(t, dir, "--db", dbFile, "phase", "next")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit when all phases resolved")
	}
}

func TestQueryUnresolvedWithPhaseFlag(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-001", "--title", "In-scope risk")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-002", "--title", "Out-of-scope risk")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "question", "--id", "QST-001", "--title", "In-scope question")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "RSK-001", "--type", "covers")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "QST-001", "--type", "covers")

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved", "--phase", "PHS-001")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryUnresolvedResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Summary.Total != 2 {
		t.Errorf("total = %d; want 2 (RSK-001 + QST-001, not RSK-002)", resp.Summary.Total)
	}

	for _, e := range resp.Entities {
		if e.ID == "RSK-002" {
			t.Error("RSK-002 should be excluded (not in phase scope)")
		}
	}
}

func TestQueryUnresolvedWithoutPhaseReturnsAll(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-001", "--title", "Risk 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "risk", "--id", "RSK-002", "--title", "Risk 2")

	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "RSK-001", "--type", "covers")

	r := runCLI(t, dir, "--db", dbFile, "query", "unresolved")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.QueryUnresolvedResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if resp.Summary.Total != 2 {
		t.Errorf("total = %d; want 2 (both risks without --phase filter)", resp.Summary.Total)
	}
}

func TestAutoActivateOnDelivers(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Auth")

	r := runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("get before: exit %d", r.exitCode)
	}
	var before jsoncontract.EntityResponse
	json.Unmarshal([]byte(r.stdout), &before)
	if before.Entity.Status != model.EntityStatusDraft {
		t.Fatalf("precondition: REQ-001 should be draft, got %q", before.Entity.Status)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "REQ-001", "--type", "delivers")
	if r.exitCode != 0 {
		t.Fatalf("relation add delivers: exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("get after: exit %d", r.exitCode)
	}
	var after jsoncontract.EntityResponse
	json.Unmarshal([]byte(r.stdout), &after)
	if after.Entity.Status != model.EntityStatusActive {
		t.Errorf("REQ-001 status after delivers = %q; want active", after.Entity.Status)
	}
}

func TestAutoActivateSkipsAlreadyActive(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Auth", "--status", "active")

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "REQ-001", "--type", "delivers")
	if r.exitCode != 0 {
		t.Fatalf("relation add delivers: exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("get after: exit %d", r.exitCode)
	}
	var after jsoncontract.EntityResponse
	json.Unmarshal([]byte(r.stdout), &after)
	if after.Entity.Status != model.EntityStatusActive {
		t.Errorf("REQ-001 status = %q; want active (unchanged)", after.Entity.Status)
	}
}

func TestAutoActivateDoesNotResolve(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Use JWT")

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "DEC-001", "--type", "delivers")
	if r.exitCode != 0 {
		t.Fatalf("relation add delivers: exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "DEC-001")
	if r.exitCode != 0 {
		t.Fatalf("get after: exit %d", r.exitCode)
	}
	var after jsoncontract.EntityResponse
	json.Unmarshal([]byte(r.stdout), &after)
	if after.Entity.Status == model.EntityStatusResolved {
		t.Error("DEC-001 should NOT be auto-resolved, only auto-activated to active")
	}
	if after.Entity.Status != model.EntityStatusActive {
		t.Errorf("DEC-001 status = %q; want active", after.Entity.Status)
	}
}

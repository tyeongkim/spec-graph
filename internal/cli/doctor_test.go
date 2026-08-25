package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func runDoctorCheck(t *testing.T, dir, dbFile, check string) jsoncontract.DoctorCheck {
	t.Helper()
	r := runCLI(t, dir, "--db", dbFile, "doctor", "--check", check)

	var report jsoncontract.DoctorReport
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("unmarshal doctor report: %v\nraw: %s", err, r.stdout)
	}
	for _, c := range report.Checks {
		if c.Name == check {
			return c
		}
	}
	t.Fatalf("doctor did not report check %q; raw: %s", check, r.stdout)
	return jsoncontract.DoctorCheck{}
}

func TestDoctorAcceptsSupersedesOwnedByLexicographicallyLargerRevision(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	for _, e := range []struct{ id, title string }{
		{"REQ-001", "Mention payload API"},
		{"REQ-002", "Mention resolution API"},
	} {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "requirement", "--id", e.id, "--title", e.title)
		if r.exitCode != 0 {
			t.Fatalf("entity add %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}

	r := runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-002", "--to", "REQ-001", "--type", "supersedes")
	if r.exitCode != 0 {
		t.Fatalf("relation add supersedes: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	if r := runCLI(t, dir, "--db", dbFile, "entity", "deprecate", "REQ-001"); r.exitCode != 0 {
		t.Fatalf("entity deprecate REQ-001: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	owner := filepath.Join(filepath.Dir(dbFile), "entities", "requirement", "REQ-002.toml")
	stored, err := os.ReadFile(owner)
	if err != nil {
		t.Fatalf("read revision file: %v", err)
	}
	if !strings.Contains(string(stored), `to = "REQ-001"`) {
		t.Fatalf("supersedes edge not stored in the revision's file; REQ-002.toml =\n%s", stored)
	}

	check := runDoctorCheck(t, dir, dbFile, "symmetric_relations")
	if check.Status != "pass" {
		t.Errorf("symmetric_relations = %q, issues %+v; supersedes is directional and belongs in the superseding entity's file",
			check.Status, check.Issues)
	}
}

func TestDoctorFlagsConflictsWithStoredInLargerIDFile(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	for _, id := range []string{"DEC-001", "DEC-002"} {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "decision", "--id", id, "--title", id)
		if r.exitCode != 0 {
			t.Fatalf("entity add %s: exit=%d stderr=%s", id, r.exitCode, r.stderr)
		}
	}

	larger := filepath.Join(filepath.Dir(dbFile), "entities", "decision", "DEC-002.toml")
	existing, err := os.ReadFile(larger)
	if err != nil {
		t.Fatalf("read DEC-002: %v", err)
	}
	reversed := string(existing) + "\n[[relations]]\nto = \"DEC-001\"\ntype = \"conflicts_with\"\n"
	if err := os.WriteFile(larger, []byte(reversed), 0o644); err != nil {
		t.Fatalf("write DEC-002: %v", err)
	}

	check := runDoctorCheck(t, dir, dbFile, "symmetric_relations")
	if check.Status != "fail" {
		t.Errorf("symmetric_relations = %q; want fail, conflicts_with is undirected and must sit in the smaller ID's file", check.Status)
	}
}

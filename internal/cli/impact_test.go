package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

func seedImpactEntities(t *testing.T, dbFile string) {
	t.Helper()
	dir := t.TempDir()

	entities := []struct {
		typ, id, title string
	}{
		{"requirement", "REQ-001", "Auth requirement"},
		{"decision", "DEC-003", "Use JWT"},
		{"interface", "API-005", "Auth endpoint"},
		{"test", "TST-010", "Auth test"},
	}
	for _, e := range entities {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", e.typ, "--id", e.id, "--title", e.title)
		if r.exitCode != 0 {
			t.Fatalf("seed entity %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
	}
}

func seedImpactRelations(t *testing.T, dbFile string, relations []struct{ from, to, relType string }) {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range relations {
		r := runCLI(t, dir, "--db", dbFile, "relation", "add",
			"--from", rel.from, "--to", rel.to, "--type", rel.relType)
		if r.exitCode != 0 {
			t.Fatalf("seed relation %s->%s (%s): exit=%d stderr=%s",
				rel.from, rel.to, rel.relType, r.exitCode, r.stderr)
		}
	}
}

func TestImpactSingleSource(t *testing.T) {
	dbFile := initTestProject(t)
	seedImpactEntities(t, dbFile)

	relations := []struct{ from, to, relType string }{
		{"API-005", "REQ-001", "implements"},
	}
	seedImpactRelations(t, dbFile, relations)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if len(resp.Sources) != 1 || resp.Sources[0] != "REQ-001" {
		t.Errorf("sources=%v; want [REQ-001]", resp.Sources)
	}

	found := false
	for _, a := range resp.Affected {
		if a.ID == "API-005" {
			found = true
			if a.Type != "interface" {
				t.Errorf("API-005 type=%q; want interface", a.Type)
			}
		}
	}
	if !found {
		t.Errorf("expected API-005 in affected, got %+v", resp.Affected)
	}

	if resp.Summary.Total != len(resp.Affected) {
		t.Errorf("summary.total=%d; want %d", resp.Summary.Total, len(resp.Affected))
	}
}

func TestImpactMultipleSources(t *testing.T) {
	dbFile := initTestProject(t)
	seedImpactEntities(t, dbFile)

	relations := []struct{ from, to, relType string }{
		{"API-005", "REQ-001", "implements"},
		{"REQ-001", "DEC-003", "depends_on"},
	}
	seedImpactRelations(t, dbFile, relations)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "REQ-001", "DEC-003")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if len(resp.Sources) != 2 {
		t.Errorf("sources count=%d; want 2", len(resp.Sources))
	}

	sourceSet := map[string]bool{}
	for _, s := range resp.Sources {
		sourceSet[s] = true
	}
	if !sourceSet["REQ-001"] || !sourceSet["DEC-003"] {
		t.Errorf("sources=%v; want REQ-001 and DEC-003", resp.Sources)
	}
}

func TestImpactWithFollowFlag(t *testing.T) {
	dbFile := initTestProject(t)
	seedImpactEntities(t, dbFile)

	relations := []struct{ from, to, relType string }{
		{"API-005", "REQ-001", "implements"},
		{"TST-010", "REQ-001", "verifies"},
	}
	seedImpactRelations(t, dbFile, relations)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "REQ-001", "--follow", "implements")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	for _, a := range resp.Affected {
		if a.ID == "TST-010" {
			t.Errorf("TST-010 should be excluded when --follow=implements, but found in affected")
		}
	}

	found := false
	for _, a := range resp.Affected {
		if a.ID == "API-005" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected API-005 in affected with --follow=implements")
	}
}

func TestImpactWithMinSeverity(t *testing.T) {
	dbFile := initTestProject(t)
	seedImpactEntities(t, dbFile)

	relations := []struct{ from, to, relType string }{
		{"API-005", "REQ-001", "implements"},
	}
	seedImpactRelations(t, dbFile, relations)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "REQ-001", "--min-severity", "high")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	for _, a := range resp.Affected {
		if a.Impact.Overall != "high" {
			t.Errorf("affected %s has overall=%q; want high (--min-severity high)", a.ID, a.Impact.Overall)
		}
	}
}

func TestImpactWithDimension(t *testing.T) {
	dbFile := initTestProject(t)
	seedImpactEntities(t, dbFile)

	relations := []struct{ from, to, relType string }{
		{"API-005", "REQ-001", "implements"},
	}
	seedImpactRelations(t, dbFile, relations)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "REQ-001", "--dimension", "structural")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if !json.Valid([]byte(r.stdout)) {
		t.Error("output is not valid JSON")
	}
}

func TestImpactEntityNotFound(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact", "NONEXIST-001")
	if r.exitCode == 0 {
		t.Fatalf("expected non-zero exit for nonexistent entity, got 0")
	}
}

func TestImpactNoRelations(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-050", "--title", "Isolated")
	if r.exitCode != 0 {
		t.Fatalf("seed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "impact", "REQ-050")
	if r.exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.ImpactResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}

	if len(resp.Affected) != 0 {
		t.Errorf("affected count=%d; want 0", len(resp.Affected))
	}
	if resp.Summary.Total != 0 {
		t.Errorf("summary.total=%d; want 0", resp.Summary.Total)
	}
}

func TestImpactInvalidFlag(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-060", "--title", "Test")
	if r.exitCode != 0 {
		t.Fatalf("seed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "impact", "REQ-060", "--min-severity", "invalid")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3 for invalid flag, got %d; stderr=%s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("stderr not valid error JSON: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code=%q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestImpactNoArgs(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "impact")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing args")
	}
}

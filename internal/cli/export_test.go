package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

func TestExportDOT(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "First Req")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "First Dec")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "DEC-001", "--type", "decides")

	r := runCLI(t, dir, "--db", dbFile, "export", "--format", "dot")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	if !strings.Contains(r.stdout, "digraph spec_graph {") {
		t.Errorf("expected 'digraph spec_graph {' in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "REQ-001") {
		t.Errorf("expected REQ-001 in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "DEC-001") {
		t.Errorf("expected DEC-001 in output, got:\n%s", r.stdout)
	}
}

func TestExportMermaid(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "First Req")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "First Dec")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "DEC-001", "--type", "decides")

	r := runCLI(t, dir, "--db", dbFile, "export", "--format", "mermaid")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	if !strings.Contains(r.stdout, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "REQ-001") {
		t.Errorf("expected REQ-001 in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "DEC-001") {
		t.Errorf("expected DEC-001 in output, got:\n%s", r.stdout)
	}
}

func TestExportInvalidFormat(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "export", "--format", "xml")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	if !strings.Contains(r.stderr, "INVALID_INPUT") {
		t.Errorf("expected INVALID_INPUT in stderr, got:\n%s", r.stderr)
	}
}

func TestExportMissingFormat(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "export")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required --format flag")
	}
}

func TestExportCenterSubgraph(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req One")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Dec One")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "interface", "--id", "API-001", "--title", "Interface One")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "DEC-001", "--type", "depends_on")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "DEC-001", "--to", "API-001", "--type", "references")

	r := runCLI(t, dir, "--db", dbFile, "export", "--format", "dot",
		"--center", "DEC-001", "--depth", "1")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "DEC-001") {
		t.Errorf("expected center DEC-001 in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "REQ-001") {
		t.Errorf("expected neighbor REQ-001 in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "API-001") {
		t.Errorf("expected neighbor API-001 in output, got:\n%s", r.stdout)
	}

	r = runCLI(t, dir, "--db", dbFile, "export", "--format", "mermaid",
		"--center", "REQ-001", "--depth", "1")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "REQ-001") {
		t.Errorf("expected center REQ-001 in output, got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "DEC-001") {
		t.Errorf("expected neighbor DEC-001 in output, got:\n%s", r.stdout)
	}
	if strings.Contains(r.stdout, "API-001") {
		t.Errorf("expected API-001 to be excluded at depth 1, got:\n%s", r.stdout)
	}
}

func TestExportCenterNonexistent(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "export", "--format", "dot",
		"--center", "NOPE-999")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1 for nonexistent center, got %d; stderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "ENTITY_NOT_FOUND") {
		t.Errorf("expected ENTITY_NOT_FOUND in stderr, got:\n%s", r.stderr)
	}
}

func TestExportEmptyDB(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "export", "--format", "dot")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "digraph spec_graph {") {
		t.Errorf("expected valid empty DOT graph, got:\n%s", r.stdout)
	}

	r = runCLI(t, t.TempDir(), "--db", dbFile, "export", "--format", "mermaid")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "flowchart LR") {
		t.Errorf("expected valid empty Mermaid graph, got:\n%s", r.stdout)
	}
}

func TestExportJSON(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "First Req")
	runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "First Dec")
	runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "REQ-001", "--to", "DEC-001", "--type", "depends_on")

	r := runCLI(t, dir, "--db", dbFile, "export", "--format", "json")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var result jsoncontract.ExportJSONResult
	if err := json.Unmarshal([]byte(r.stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, r.stdout)
	}

	if len(result.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(result.Entities))
	}
	if result.Entities[0].ID != "DEC-001" {
		t.Errorf("expected first entity DEC-001 (sorted), got %s", result.Entities[0].ID)
	}
	if result.Entities[1].ID != "REQ-001" {
		t.Errorf("expected second entity REQ-001 (sorted), got %s", result.Entities[1].ID)
	}
	if result.Entities[0].Type != "decision" {
		t.Errorf("expected type 'decision', got %q", result.Entities[0].Type)
	}
	if result.Entities[0].Metadata == nil {
		t.Error("expected non-nil metadata map")
	}

	if len(result.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(result.Relations))
	}
	rel := result.Relations[0]
	if rel.FromID != "REQ-001" || rel.ToID != "DEC-001" || rel.Type != "depends_on" {
		t.Errorf("unexpected relation: %+v", rel)
	}
}

func TestExportJSONEmpty(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "export", "--format", "json")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var result jsoncontract.ExportJSONResult
	if err := json.Unmarshal([]byte(r.stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, r.stdout)
	}

	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result.Entities))
	}
	if len(result.Relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(result.Relations))
	}
}

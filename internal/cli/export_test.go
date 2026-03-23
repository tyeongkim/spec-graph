package cli_test

import (
	"strings"
	"testing"
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

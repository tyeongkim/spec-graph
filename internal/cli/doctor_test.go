package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func runDoctorCheck(t *testing.T, dir, dbFile, check string) (cliResult, jsoncontract.DoctorReport, jsoncontract.DoctorCheck) {
	t.Helper()
	r := runCLI(t, dir, "--db", dbFile, "doctor", "--check", check)

	var report jsoncontract.DoctorReport
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("unmarshal doctor report: %v\nraw: %s", err, r.stdout)
	}
	for _, c := range report.Checks {
		if c.Name == check {
			return r, report, c
		}
	}
	t.Fatalf("doctor did not report check %q; raw: %s", check, r.stdout)
	return cliResult{}, jsoncontract.DoctorReport{}, jsoncontract.DoctorCheck{}
}

func addDoctorEntity(t *testing.T, dir, dbFile, entityType, id string) string {
	t.Helper()
	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", entityType, "--id", id, "--title", id)
	if r.exitCode != 0 {
		t.Fatalf("add %s %s: exit=%d stderr=%s", entityType, id, r.exitCode, r.stderr)
	}
	return filepath.Join(filepath.Dir(dbFile), "entities", entityType, id+".toml")
}

func replaceDoctorFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	updated := strings.Replace(string(data), old, new, 1)
	if updated == string(data) {
		t.Fatalf("replace %q in %s", old, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func appendDoctorRelation(t *testing.T, path, to, relationType string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	data = append(data, []byte("\n[[relations]]\nto = \""+to+"\"\ntype = \""+relationType+"\"\n")...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
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

	if r := runCLI(t, dir, "--db", dbFile, "entity", "update", "REQ-001",
		"--status", "deprecated", "--reason", "superseded by REQ-002"); r.exitCode != 0 {
		t.Fatalf("entity update REQ-001 --status deprecated: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	owner := filepath.Join(filepath.Dir(dbFile), "entities", "requirement", "REQ-002.toml")
	stored, err := os.ReadFile(owner)
	if err != nil {
		t.Fatalf("read revision file: %v", err)
	}
	if !strings.Contains(string(stored), `to = "REQ-001"`) {
		t.Fatalf("supersedes edge not stored in the revision's file; REQ-002.toml =\n%s", stored)
	}

	_, _, check := runDoctorCheck(t, dir, dbFile, "symmetric_relations")
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

	_, _, check := runDoctorCheck(t, dir, dbFile, "symmetric_relations")
	if check.Status != "fail" {
		t.Errorf("symmetric_relations = %q; want fail, conflicts_with is undirected and must sit in the smaller ID's file", check.Status)
	}
}

func TestDoctorReportsSelectedIntegrityCheckFailures(t *testing.T) {
	tests := []struct {
		name  string
		check string
		setup func(t *testing.T, dir, dbFile string)
	}{
		{
			name:  "toml parse",
			check: "toml_parse",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				if err := os.WriteFile(path, []byte("title = ["), 0o644); err != nil {
					t.Fatalf("write malformed TOML: %v", err)
				}
			},
		},
		{
			name:  "id filename match",
			check: "id_filename_match",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				replaceDoctorFile(t, path, `id = "REQ-001"`, `id = "REQ-002"`)
			},
		},
		{
			name:  "type directory match",
			check: "type_directory_match",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				replaceDoctorFile(t, path, `type = "requirement"`, `type = "decision"`)
			},
		},
		{
			name:  "duplicate IDs",
			check: "duplicate_ids",
			setup: func(t *testing.T, dir, dbFile string) {
				addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				path := addDoctorEntity(t, dir, dbFile, "interface", "API-001")
				replaceDoctorFile(t, path, `id = "API-001"`, `id = "REQ-001"`)
			},
		},
		{
			name:  "orphan relations",
			check: "orphan_relations",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				appendDoctorRelation(t, path, "REQ-404", "references")
			},
		},
		{
			name:  "edge matrix",
			check: "edge_matrix",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				addDoctorEntity(t, dir, dbFile, "interface", "API-001")
				appendDoctorRelation(t, path, "API-001", "implements")
			},
		},
		{
			name:  "schema validation",
			check: "schema_validation",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				replaceDoctorFile(t, path, `status = "draft"`, `status = "invalid"`)
			},
		},
		{
			name:  "self loop relations",
			check: "self_loop_relations",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				appendDoctorRelation(t, path, "REQ-001", "references")
			},
		},
		{
			name:  "stale index",
			check: "stale_index",
			setup: func(t *testing.T, dir, dbFile string) {
				path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
				replaceDoctorFile(t, path, `title = "REQ-001"`, `title = "Updated requirement"`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dbFile := initTestProject(t)
			dir := t.TempDir()
			test.setup(t, dir, dbFile)

			result, report, check := runDoctorCheck(t, dir, dbFile, test.check)
			if result.exitCode != 2 {
				t.Fatalf("exit=%d; want 2\nstderr=%s\nstdout=%s", result.exitCode, result.stderr, result.stdout)
			}
			if len(report.Checks) != 1 || report.Checks[0].Name != test.check {
				t.Fatalf("checks=%+v; want only %q", report.Checks, test.check)
			}
			if report.Healthy {
				t.Error("healthy = true; want false")
			}
			if report.Summary.TotalChecks != 1 || report.Summary.Passed != 0 || report.Summary.Failed != 1 || report.Summary.TotalIssues < 1 {
				t.Errorf("summary=%+v; want one failed check with at least one issue", report.Summary)
			}
			if check.Status != "fail" {
				t.Errorf("%s status = %q; want fail", test.check, check.Status)
			}
			if len(check.Issues) == 0 {
				t.Errorf("%s issues = none; want at least one", test.check)
			}
		})
	}
}

func TestDoctorStaleIndexDoesNotModifyIndex(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	path := addDoctorEntity(t, dir, dbFile, "requirement", "REQ-001")
	replaceDoctorFile(t, path, `title = "REQ-001"`, `title = "Updated requirement"`)

	snapshot := func() map[string][]byte {
		paths, err := filepath.Glob(dbFile + "*")
		if err != nil {
			t.Fatalf("list index files: %v", err)
		}
		files := make(map[string][]byte, len(paths))
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read index file %s: %v", path, err)
			}
			files[path] = data
		}
		return files
	}

	before := snapshot()
	if _, ok := before[dbFile]; !ok {
		t.Fatal("graph.db is missing before doctor")
	}

	result, _, check := runDoctorCheck(t, dir, dbFile, "stale_index")
	if result.exitCode != 2 {
		t.Fatalf("exit=%d; want 2\nstderr=%s\nstdout=%s", result.exitCode, result.stderr, result.stdout)
	}
	if check.Status != "fail" {
		t.Fatalf("stale_index status = %q; want fail", check.Status)
	}
	if len(check.Issues) != 1 || !strings.HasPrefix(check.Issues[0].Message, "index is stale:") {
		t.Fatalf("stale_index issues = %+v; want stale fingerprint issue", check.Issues)
	}

	after := snapshot()
	for path, beforeData := range before {
		afterData, ok := after[path]
		if !ok {
			t.Errorf("index file %s was removed", path)
			continue
		}
		if !bytes.Equal(afterData, beforeData) {
			t.Errorf("index file %s changed", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("index file %s was created", path)
		}
	}
}

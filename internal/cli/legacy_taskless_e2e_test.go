package cli_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestE2E_LegacyTasklessWorkflow(t *testing.T) {
	projectDir := t.TempDir()
	legacyDir := filepath.Join(projectDir, "plans")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("create legacy plan directory: %v", err)
	}
	legacyPath := filepath.Join(legacyDir, "phase-1.md")
	legacyBytes := []byte("# Legacy Phase 1\n\nKeep this plan unchanged.\n")
	if err := os.WriteFile(legacyPath, legacyBytes, 0o644); err != nil {
		t.Fatalf("write legacy plan: %v", err)
	}

	dbFile := filepath.Join(projectDir, ".spec-graph", "graph.db")
	result := runCLI(t, projectDir, "init", "--db", dbFile)
	if result.exitCode != 0 {
		t.Fatalf("init: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	entities := [][]string{
		{"plan", "PLN-001", "Legacy plan", `{"owner":"legacy"}`},
		{"phase", "PHS-001", "Legacy phase", `{"goal":"Preserve legacy scope","order":1,"exit_criteria":["Legacy behavior is unchanged"]}`},
		{"requirement", "REQ-001", "Legacy requirement", `{"priority":"must","kind":"functional"}`},
	}
	for _, entity := range entities {
		result = runCLI(t, projectDir, "--db", dbFile, "entity", "add", "--type", entity[0], "--id", entity[1], "--title", entity[2], "--metadata", entity[3])
		if result.exitCode != 0 {
			t.Fatalf("entity add %s: exit=%d stderr=%s", entity[1], result.exitCode, result.stderr)
		}
	}
	for _, relation := range [][]string{{"PHS-001", "PLN-001", "belongs_to"}, {"PHS-001", "REQ-001", "covers"}} {
		result = runCLI(t, projectDir, "--db", dbFile, "relation", "add", "--from", relation[0], "--to", relation[1], "--type", relation[2])
		if result.exitCode != 0 {
			t.Fatalf("relation add %v: exit=%d stderr=%s", relation, result.exitCode, result.stderr)
		}
	}

	result = runCLI(t, projectDir, "--db", dbFile, "entity", "list")
	if result.exitCode != 0 {
		t.Fatalf("entity list: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	var entityList jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(result.stdout), &entityList); err != nil {
		t.Fatalf("unmarshal entity list: %v", err)
	}
	if entityList.Count != 3 {
		t.Fatalf("entity count = %d; want 3", entityList.Count)
	}

	result = runCLI(t, projectDir, "--db", dbFile, "query", "scope", "PHS-001")
	if result.exitCode != 0 {
		t.Fatalf("query scope: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	var scope jsoncontract.QueryScopeResponse
	if err := json.Unmarshal([]byte(result.stdout), &scope); err != nil {
		t.Fatalf("unmarshal scope: %v", err)
	}
	if scope.Summary.Total != 1 || len(scope.Entities) != 1 || scope.Entities[0].ID != "REQ-001" || len(scope.Relations) != 1 || scope.Relations[0].FromID != "PHS-001" {
		t.Fatalf("legacy direct scope changed: %+v", scope)
	}

	result = runCLI(t, projectDir, "--db", dbFile, "phase", "context", "PHS-001")
	if result.exitCode != 0 {
		t.Fatalf("phase context: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	var context specgraph.PhaseContextResult
	if err := json.Unmarshal([]byte(result.stdout), &context); err != nil {
		t.Fatalf("unmarshal phase context: %v", err)
	}
	if len(context.Tasks) != 0 || len(context.Scope) != 1 || context.Scope[0].ID != "REQ-001" {
		t.Fatalf("taskless phase context changed: %+v", context)
	}

	tomlCount, markdownCount := 0, 0
	if err := filepath.WalkDir(projectDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".toml":
			tomlCount++
		case ".md":
			markdownCount++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk project: %v", err)
	}
	if tomlCount != 3 || markdownCount != 1 {
		t.Fatalf("artifact counts = %d TOML/%d Markdown; want 3/1", tomlCount, markdownCount)
	}
	gotLegacyBytes, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("read legacy plan: %v", err)
	}
	if string(gotLegacyBytes) != string(legacyBytes) {
		t.Fatalf("legacy markdown changed: got %q want %q", gotLegacyBytes, legacyBytes)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".spec-graph", "entities", "task")); !os.IsNotExist(err) {
		t.Fatalf("task artifacts created for legacy phase: %v", err)
	}
}

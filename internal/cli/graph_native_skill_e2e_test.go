package cli_test

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestE2E_GraphNativeSkillWorkflow(t *testing.T) {
	projectDir := t.TempDir()
	dbFile := filepath.Join(projectDir, ".spec-graph", "graph.db")
	result := runCLI(t, projectDir, "init", "--db", dbFile)
	if result.exitCode != 0 {
		t.Fatalf("init: exit=%d stderr=%s", result.exitCode, result.stderr)
	}

	entities := [][]string{
		{"plan", "PLN-001", "Graph-native plan", "Graph-native delivery plan.", `{"owner":"platform"}`, "active"},
		{"phase", "PHS-001", "Graph-native phase", "Deliver graph-native planning.", `{"goal":"Deliver graph-native planning","order":1,"exit_criteria":["Context is complete"]}`, ""},
		{"requirement", "REQ-001", "Planner requirement", "Create graph-native tasks.", `{"priority":"must","kind":"functional"}`, ""},
		{"requirement", "REQ-002", "Executor requirement", "Consume task context.", `{"priority":"must","kind":"functional"}`, ""},
		{"task", "TSK-001", "Create graph tasks", "Create the phase task graph.", taskContractJSON(1, "Create the phase task graph."), ""},
		{"task", "TSK-002", "Consume phase context", "Consume the complete phase context.", taskContractJSON(2, "Consume the complete phase context."), ""},
	}
	for _, entity := range entities {
		args := []string{"--db", dbFile, "entity", "add", "--type", entity[0], "--id", entity[1], "--title", entity[2], "--description", entity[3], "--metadata", entity[4]}
		if entity[5] != "" {
			args = append(args, "--status", entity[5])
		}
		result = runCLI(t, projectDir, args...)
		if result.exitCode != 0 {
			t.Fatalf("entity add %s: exit=%d stderr=%s", entity[1], result.exitCode, result.stderr)
		}
	}

	relations := [][]string{
		{"PHS-001", "PLN-001", "belongs_to"},
		{"TSK-001", "PHS-001", "belongs_to"},
		{"TSK-002", "PHS-001", "belongs_to"},
		{"TSK-001", "REQ-001", "covers"},
		{"TSK-002", "REQ-002", "covers"},
		{"TSK-002", "TSK-001", "task_depends_on"},
	}
	for _, relation := range relations {
		result = runCLI(t, projectDir, "--db", dbFile, "relation", "add", "--from", relation[0], "--to", relation[1], "--type", relation[2])
		if result.exitCode != 0 {
			t.Fatalf("relation add %v: exit=%d stderr=%s", relation, result.exitCode, result.stderr)
		}
	}

	result = runCLI(t, projectDir, "--db", dbFile, "phase", "context", "PHS-001")
	if result.exitCode != 0 {
		t.Fatalf("phase context: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	var context specgraph.PhaseContextResult
	if err := json.Unmarshal([]byte(result.stdout), &context); err != nil {
		t.Fatalf("unmarshal phase context: %v\nraw: %s", err, result.stdout)
	}
	if context.Plan.ID != "PLN-001" || context.Phase.ID != "PHS-001" {
		t.Fatalf("context parents = %s/%s; want PLN-001/PHS-001", context.Plan.ID, context.Phase.ID)
	}
	if len(context.Tasks) != 2 || context.Tasks[0].Entity.ID != "TSK-001" || context.Tasks[1].Entity.ID != "TSK-002" {
		t.Fatalf("context tasks = %+v; want TSK-001 then TSK-002", context.Tasks)
	}
	if context.Tasks[0].Contract.Order != 1 || len(context.Tasks[0].Contract.QA) != 1 {
		t.Fatalf("first task contract incomplete: %+v", context.Tasks[0].Contract)
	}
	if len(context.Scope) != 2 || context.Scope[0].ID != "REQ-001" || context.Scope[1].ID != "REQ-002" {
		t.Fatalf("context scope = %+v; want REQ-001 and REQ-002", context.Scope)
	}
	if len(context.ReadyTaskIDs) != 1 || context.ReadyTaskIDs[0] != "TSK-001" {
		t.Fatalf("ready tasks = %v; want [TSK-001]", context.ReadyTaskIDs)
	}
	if len(context.BlockedTaskIDs) != 1 || context.BlockedTaskIDs[0] != "TSK-002" || len(context.Blockers["TSK-002"]) != 1 || context.Blockers["TSK-002"][0] != "TSK-001" {
		t.Fatalf("blocked tasks = %v blockers=%v; want TSK-002 blocked by TSK-001", context.BlockedTaskIDs, context.Blockers)
	}

	markdownCount := 0
	if err := filepath.WalkDir(projectDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(path) == ".md" {
			markdownCount++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk project: %v", err)
	}
	if markdownCount != 0 {
		t.Fatalf("markdown files created = %d; want 0", markdownCount)
	}
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// TestE2E_GraphNativeManualSurface drives the full graph-native task lifecycle
// through the CLI black-box surface with a fixture whose task_scope and delivery
// gates are satisfied legitimately (every task both covers and delivers an arch
// entity). No gate is bypassed with --force at any point.
func TestE2E_GraphNativeManualSurface(t *testing.T) {
	projectDir := t.TempDir()
	dbFile := filepath.Join(projectDir, ".spec-graph", "graph.db")

	mustCLI := func(name string, args ...string) cliResult {
		t.Helper()
		result := runCLI(t, projectDir, args...)
		if result.exitCode != 0 {
			t.Fatalf("%s: exit=%d stderr=%s", name, result.exitCode, result.stderr)
		}
		return result
	}

	mustCLI("init", "init", "--db", dbFile)

	entities := [][]string{
		{"plan", "PLN-001", "Manual surface plan", "Graph-native manual surface plan.", `{"owner":"platform"}`, "active"},
		{"phase", "PHS-001", "Manual surface phase", "Deliver the manual surface phase.", `{"goal":"Deliver manual surface","order":1,"exit_criteria":["Lifecycle resolves"]}`, ""},
		{"requirement", "REQ-001", "First requirement", "First covered requirement.", `{"priority":"must","kind":"functional"}`, ""},
		{"requirement", "REQ-002", "Second requirement", "Second covered requirement.", `{"priority":"must","kind":"functional"}`, ""},
		{"task", "TSK-001", "Implement first requirement", "Implement the first requirement.", manualTaskContractJSON(1, ""), ""},
		{"task", "TSK-002", "Implement second requirement", "Implement the second requirement.", manualTaskContractJSON(2, ""), ""},
	}
	for _, entity := range entities {
		args := []string{"--db", dbFile, "entity", "add", "--type", entity[0], "--id", entity[1], "--title", entity[2], "--description", entity[3], "--metadata", entity[4]}
		if entity[5] != "" {
			args = append(args, "--status", entity[5])
		}
		mustCLI("entity add "+entity[1], args...)
	}

	relations := [][]string{
		{"PHS-001", "PLN-001", "belongs_to"},
		{"TSK-001", "PHS-001", "belongs_to"},
		{"TSK-002", "PHS-001", "belongs_to"},
		{"TSK-001", "REQ-001", "covers"},
		{"TSK-002", "REQ-002", "covers"},
		{"TSK-001", "REQ-001", "delivers"},
		{"TSK-002", "REQ-002", "delivers"},
		{"TSK-002", "TSK-001", "task_depends_on"},
	}
	for _, relation := range relations {
		mustCLI("relation add", "--db", dbFile, "relation", "add", "--from", relation[0], "--to", relation[1], "--type", relation[2])
	}

	// Activate the phase (its parent plan is active) so tasks can later be
	// activated against an active parent phase.
	mustCLI("activate PHS-001", "--db", dbFile, "entity", "update", "PHS-001", "--status", "active")

	// phase context reflects the derived task graph, scope, and readiness.
	result := mustCLI("phase context", "--db", dbFile, "phase", "context", "PHS-001")
	var phaseContext specgraph.PhaseContextResult
	if err := json.Unmarshal([]byte(result.stdout), &phaseContext); err != nil {
		t.Fatalf("unmarshal phase context: %v\nraw: %s", err, result.stdout)
	}
	if phaseContext.Plan.ID != "PLN-001" || phaseContext.Phase.ID != "PHS-001" {
		t.Fatalf("context parents = %s/%s; want PLN-001/PHS-001", phaseContext.Plan.ID, phaseContext.Phase.ID)
	}
	if len(phaseContext.Tasks) != 2 || phaseContext.Tasks[0].Entity.ID != "TSK-001" || phaseContext.Tasks[1].Entity.ID != "TSK-002" {
		t.Fatalf("context tasks = %+v; want TSK-001 then TSK-002", phaseContext.Tasks)
	}
	if phaseContext.Tasks[0].Contract.Order != 1 || len(phaseContext.Tasks[0].Contract.QA) != 1 {
		t.Fatalf("first task contract incomplete: %+v", phaseContext.Tasks[0].Contract)
	}
	if len(phaseContext.Scope) != 2 {
		t.Fatalf("context scope = %+v; want two entries", phaseContext.Scope)
	}
	scopeIDs := map[string]bool{}
	for _, entity := range phaseContext.Scope {
		scopeIDs[entity.ID] = true
	}
	if !scopeIDs["REQ-001"] || !scopeIDs["REQ-002"] {
		t.Fatalf("context scope missing REQ-001/REQ-002: %+v", phaseContext.Scope)
	}
	if len(phaseContext.ReadyTaskIDs) != 1 || phaseContext.ReadyTaskIDs[0] != "TSK-001" {
		t.Fatalf("ready tasks = %v; want [TSK-001]", phaseContext.ReadyTaskIDs)
	}
	if len(phaseContext.BlockedTaskIDs) != 1 || phaseContext.BlockedTaskIDs[0] != "TSK-002" {
		t.Fatalf("blocked tasks = %v; want [TSK-002]", phaseContext.BlockedTaskIDs)
	}
	if len(phaseContext.Blockers["TSK-002"]) != 1 || phaseContext.Blockers["TSK-002"][0] != "TSK-001" {
		t.Fatalf("blockers = %v; want TSK-002 blocked by TSK-001", phaseContext.Blockers)
	}

	// MALFORMED: a task add carrying an unknown metadata key must be rejected and
	// leave the entity count unchanged.
	countBefore := manualEntityCount(t, projectDir, dbFile)
	malformed := `{"order":3,"instructions":["do"],"acceptance":["ok"],"must_not":[],"references":[],"qa":[{"command":"go test ./...","expected":"exit 0","evidence":""}],"agent":"x"}`
	if bad := runCLI(t, projectDir, "--db", dbFile, "entity", "add", "--type", "task", "--id", "TSK-003", "--title", "Malformed", "--description", "Unknown key.", "--metadata", malformed); bad.exitCode == 0 {
		t.Fatalf("malformed task add should fail; stdout=%s", bad.stdout)
	}
	if countAfter := manualEntityCount(t, projectDir, dbFile); countAfter != countBefore {
		t.Fatalf("entity count changed after malformed add: before=%d after=%d", countBefore, countAfter)
	}

	// Activate TSK-001; it has no prerequisites.
	mustCLI("activate TSK-001", "--db", dbFile, "entity", "update", "TSK-001", "--status", "active")

	// BLOCKED: TSK-002 cannot activate while its prerequisite TSK-001 is only
	// active (not resolved). The activation gate blocks it and it stays draft.
	blocked := runCLI(t, projectDir, "--db", dbFile, "entity", "update", "TSK-002", "--status", "active")
	if blocked.exitCode == 0 {
		t.Fatalf("TSK-002 activation should be blocked while TSK-001 unresolved; stdout=%s", blocked.stdout)
	}
	if status := manualEntityStatus(t, projectDir, dbFile, "TSK-002"); status != model.EntityStatusDraft {
		t.Fatalf("TSK-002 status = %q after blocked activation; want draft", status)
	}

	// Create repository-relative evidence files under the project root.
	evidenceDir := filepath.Join(projectDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}
	for _, name := range []string{"tsk-001.txt", "tsk-002.txt"} {
		if err := os.WriteFile(filepath.Join(evidenceDir, name), []byte("qa evidence for "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write evidence %s: %v", name, err)
		}
	}

	// Resolve TSK-001 with QA evidence set to the repo-relative file path.
	mustCLI("resolve TSK-001", "--db", dbFile, "entity", "update", "TSK-001", "--status", "resolved", "--metadata", manualTaskContractJSON(1, "evidence/tsk-001.txt"))
	if status := manualEntityStatus(t, projectDir, dbFile, "TSK-001"); status != model.EntityStatusResolved {
		t.Fatalf("TSK-001 status = %q; want resolved", status)
	}

	// With TSK-001 resolved, TSK-002 may activate and then resolve.
	mustCLI("activate TSK-002", "--db", dbFile, "entity", "update", "TSK-002", "--status", "active")
	mustCLI("resolve TSK-002", "--db", dbFile, "entity", "update", "TSK-002", "--status", "resolved", "--metadata", manualTaskContractJSON(2, "evidence/tsk-002.txt"))
	if status := manualEntityStatus(t, projectDir, dbFile, "TSK-002"); status != model.EntityStatusResolved {
		t.Fatalf("TSK-002 status = %q; want resolved", status)
	}

	// Resolve the phase, then the plan; both gates pass on the legitimate fixture.
	mustCLI("resolve PHS-001", "--db", dbFile, "entity", "update", "PHS-001", "--status", "resolved")
	if status := manualEntityStatus(t, projectDir, dbFile, "PHS-001"); status != model.EntityStatusResolved {
		t.Fatalf("PHS-001 status = %q; want resolved", status)
	}
	mustCLI("resolve PLN-001", "--db", dbFile, "entity", "update", "PLN-001", "--status", "resolved")
	if status := manualEntityStatus(t, projectDir, dbFile, "PLN-001"); status != model.EntityStatusResolved {
		t.Fatalf("PLN-001 status = %q; want resolved", status)
	}
}

// manualTaskContractJSON renders a closed TaskContract with the given order. When
// evidence is non-empty the single QA item carries it (used at resolution).
func manualTaskContractJSON(order int, evidence string) string {
	payload, _ := json.Marshal(map[string]any{
		"order":        order,
		"instructions": []string{"Implement the scoped behavior."},
		"acceptance":   []string{"The behavior is verified."},
		"must_not":     []string{},
		"references":   []string{},
		"qa":           []map[string]string{{"command": "go test ./...", "expected": "exit 0", "evidence": evidence}},
	})
	return string(payload)
}

func manualEntityCount(t *testing.T, dir, dbFile string) int {
	t.Helper()
	result := runCLI(t, dir, "--db", dbFile, "entity", "list")
	if result.exitCode != 0 {
		t.Fatalf("entity list: exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	var list jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(result.stdout), &list); err != nil {
		t.Fatalf("unmarshal entity list: %v\nraw: %s", err, result.stdout)
	}
	return list.Count
}

func manualEntityStatus(t *testing.T, dir, dbFile, id string) model.EntityStatus {
	t.Helper()
	result := runCLI(t, dir, "--db", dbFile, "entity", "get", id)
	if result.exitCode != 0 {
		t.Fatalf("entity get %s: exit=%d stderr=%s", id, result.exitCode, result.stderr)
	}
	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(result.stdout), &resp); err != nil {
		t.Fatalf("unmarshal entity get %s: %v\nraw: %s", id, err, result.stdout)
	}
	return resp.Entity.Status
}

package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestPhaseContextTransportParity(t *testing.T) {
	dbFile := initTestProject(t)
	seedPhaseContextCLI(t, dbFile)
	dir := t.TempDir()

	result := runCLI(t, dir, "--db", dbFile, "phase", "context", "PHS-001")
	if result.exitCode != 0 {
		t.Fatalf("phase context: exit %d; stderr: %s", result.exitCode, result.stderr)
	}

	engine, err := specgraph.Open(context.Background(), specgraph.Options{Root: filepath.Dir(dbFile)})
	if err != nil {
		t.Fatalf("open fixture engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	want, err := engine.PhaseContext("PHS-001")
	if err != nil {
		t.Fatalf("direct PhaseContext: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal direct PhaseContext: %v", err)
	}
	var normalizedWant any
	if err := json.Unmarshal(wantJSON, &normalizedWant); err != nil {
		t.Fatalf("normalize direct PhaseContext: %v", err)
	}

	var got any
	if err := json.Unmarshal([]byte(result.stdout), &got); err != nil {
		t.Fatalf("unmarshal CLI result: %v\nraw: %s", err, result.stdout)
	}
	if !reflect.DeepEqual(got, normalizedWant) {
		t.Errorf("CLI phase context differs from engine result\ngot:  %+v\nwant: %+v", got, normalizedWant)
	}
}

func TestPhaseContextTransportErrors(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	runCLI(t, dir, "--db", dbFile, "entity", "add", "--type", "requirement", "--id", "REQ-001", "--title", "Requirement")

	tests := []struct {
		name     string
		id       string
		wantCode string
	}{
		{name: "non-phase ID", id: "REQ-001", wantCode: "INVALID_INPUT"},
		{name: "missing ID", id: "PHS-999", wantCode: "ENTITY_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runCLI(t, dir, "--db", dbFile, "phase", "context", test.id)
			if result.exitCode == 0 {
				t.Fatal("expected phase context to fail")
			}
			if result.stdout != "" {
				t.Errorf("unexpected result payload: %s", result.stdout)
			}
			var response jsoncontract.ErrorResponse
			if err := json.Unmarshal([]byte(result.stderr), &response); err != nil {
				t.Fatalf("unmarshal CLI error: %v\nraw: %s", err, result.stderr)
			}
			if response.Error.Code != test.wantCode {
				t.Errorf("error code = %q; want %q", response.Error.Code, test.wantCode)
			}
		})
	}
}

func seedPhaseContextCLI(t *testing.T, dbFile string) {
	t.Helper()
	dir := t.TempDir()
	entities := [][]string{
		{"--type", "plan", "--id", "PLN-001", "--title", "Delivery plan", "--description", "Complete parent plan.", "--metadata", `{"owner":"platform"}`},
		{"--type", "phase", "--id", "PHS-001", "--title", "Context phase", "--description", "Complete child phase.", "--metadata", `{"goal":"Expose phase context","order":1,"exit_criteria":["Context is deterministic"]}`},
		{"--type", "requirement", "--id", "REQ-001", "--title", "First requirement", "--description", "First covered entity.", "--metadata", `{"priority":"must","kind":"functional","owner":"platform"}`},
		{"--type", "requirement", "--id", "REQ-002", "--title", "Second requirement", "--description", "Second covered entity.", "--metadata", `{"priority":"should","kind":"non_functional","owner":"quality"}`},
		{"--type", "task", "--id", "TSK-001", "--title", "Implement result", "--description", "Implement the engine result.", "--metadata", taskContractJSON(2, "Implement the engine result.")},
		{"--type", "task", "--id", "TSK-002", "--title", "Preserve entities", "--description", "Preserve complete entities.", "--metadata", taskContractJSON(1, "Preserve complete entities.")},
		{"--type", "task", "--id", "TSK-003", "--title", "Consume result", "--description", "Consume the engine result.", "--metadata", taskContractJSON(1, "Consume the engine result.")},
	}
	for _, entityArgs := range entities {
		args := append([]string{"--db", dbFile, "entity", "add"}, entityArgs...)
		result := runCLI(t, dir, args...)
		if result.exitCode != 0 {
			t.Fatalf("seed entity %v: exit %d; stderr: %s", entityArgs, result.exitCode, result.stderr)
		}
	}

	relations := [][]string{
		{"PHS-001", "PLN-001", "belongs_to"},
		{"TSK-001", "PHS-001", "belongs_to"},
		{"TSK-002", "PHS-001", "belongs_to"},
		{"TSK-003", "PHS-001", "belongs_to"},
		{"TSK-003", "TSK-001", "task_depends_on"},
		{"TSK-001", "REQ-001", "covers"},
		{"TSK-002", "REQ-002", "covers"},
		{"TSK-002", "REQ-002", "delivers"},
		{"TSK-003", "REQ-001", "covers"},
	}
	for _, relation := range relations {
		result := runCLI(t, dir, "--db", dbFile, "relation", "add", "--from", relation[0], "--to", relation[1], "--type", relation[2])
		if result.exitCode != 0 {
			t.Fatalf("seed relation %v: exit %d; stderr: %s", relation, result.exitCode, result.stderr)
		}
	}
}

func taskContractJSON(order int, instruction string) string {
	payload, _ := json.Marshal(map[string]any{
		"order": order, "instructions": []string{instruction}, "acceptance": []string{"Transport matches engine result."},
		"must_not": []string{}, "references": []string{},
		"qa": []map[string]string{{"command": "go test ./...", "expected": "exit 0", "evidence": ""}},
	})
	return string(payload)
}

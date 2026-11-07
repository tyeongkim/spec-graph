package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func TestActiveTaskContractFreezeThroughCLI(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	root := []string{"--root", filepath.Dir(dbFile)}
	run := func(step string, args ...string) cliResult {
		t.Helper()
		r := runCLI(t, dir, append(append([]string{}, root...), args...)...)
		if r.exitCode != 0 {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", step, r.exitCode, r.stdout, r.stderr)
		}
		return r
	}

	run("add plan", "entity", "add", "--type", "plan", "--id", "PLN-001", "--title", "Plan", "--status", "active")
	run("add phase", "entity", "add", "--type", "phase", "--id", "PHS-001", "--title", "Phase", "--metadata", `{"goal":"Ship","order":1}`)
	run("add requirement", "entity", "add", "--type", "requirement", "--id", "REQ-001", "--title", "Requirement", "--status", "active")
	run("add task", "entity", "add", "--type", "task", "--id", "TSK-001", "--title", "Task", "--description", "Do the work.", "--metadata", manualTaskContractJSON(1, ""))
	run("phase belongs_to", "relation", "add", "--from", "PHS-001", "--to", "PLN-001", "--type", "belongs_to")
	run("task belongs_to", "relation", "add", "--from", "TSK-001", "--to", "PHS-001", "--type", "belongs_to")
	run("task covers", "relation", "add", "--from", "TSK-001", "--to", "REQ-001", "--type", "covers")
	run("activate phase", "entity", "update", "PHS-001", "--status", "active")
	run("activate task", "entity", "update", "TSK-001", "--status", "active")

	blocked := runCLI(t, dir, append(append([]string{}, root...), "entity", "update", "TSK-001", "--title", "Renamed")...)
	if blocked.exitCode != 2 {
		t.Fatalf("rename active task: exit=%d; want 2; stdout=%s stderr=%s", blocked.exitCode, blocked.stdout, blocked.stderr)
	}
	var gate jsoncontract.EntityUpdateGateResponse
	if err := json.Unmarshal([]byte(blocked.stdout), &gate); err != nil {
		t.Fatalf("unmarshal gate response: %v\nraw: %s", err, blocked.stdout)
	}
	if !gate.Blocked || len(gate.Issues) != 1 || gate.Issues[0].Check != "frozen_fields" {
		t.Fatalf("gate response = %+v; want one frozen_fields issue", gate)
	}

	forced := run("forced rename", "entity", "update", "TSK-001", "--title", "Renamed", "--force", "--reason", "Title typo")
	var success jsoncontract.EntityUpdateSuccessResponse
	if err := json.Unmarshal([]byte(forced.stdout), &success); err != nil {
		t.Fatalf("unmarshal forced response: %v\nraw: %s", err, forced.stdout)
	}
	if success.Outcome != "applied_with_force" || !success.Entity.AmendmentForced || success.Entity.AmendmentReason != "Title typo" {
		t.Fatalf("forced response = %+v", success)
	}

	stored := run("get task", "entity", "get", "TSK-001")
	var entity jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(stored.stdout), &entity); err != nil {
		t.Fatalf("unmarshal stored task: %v", err)
	}
	if entity.Entity.Title != "Renamed" || !entity.Entity.AmendmentForced || entity.Entity.AmendmentReason != "Title typo" {
		t.Fatalf("stored task = %+v; amendment audit did not round-trip", entity.Entity)
	}
}

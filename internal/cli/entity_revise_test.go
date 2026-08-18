package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestEntityReviseMovesLiveRelationsAndKeepsDeliveryRecord(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	setupDeliveredRequirement(t, dir, dbFile)

	r := runCLI(t, dir, "--db", dbFile, "entity", "revise", "REQ-001",
		"--title", "Query operations, narrowed",
		"--reason", "Split traversal out of query scope")
	if r.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
	}

	var resp jsoncontract.EntityReviseResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal revise response: %v\nraw: %s", err, r.stdout)
	}

	if resp.Revision.ID == "REQ-001" {
		t.Fatal("revision reused the superseded ID")
	}
	if resp.Revision.Title != "Query operations, narrowed" {
		t.Errorf("revision.title = %q; want the requested title", resp.Revision.Title)
	}
	if resp.Superseded.Status != model.EntityStatusDeprecated {
		t.Errorf("superseded.status = %q; want deprecated", resp.Superseded.Status)
	}

	assertRelation(t, resp.Carried, "API-001", model.RelationImplements)
	assertRelation(t, resp.Retained, "PHS-001", model.RelationDelivers)

	validation := runCLI(t, dir, "--db", dbFile, "validate", "--check", "mapping_consistency")
	if validation.exitCode != 0 {
		t.Fatalf("mapping_consistency after revise: exit=%d stdout=%s stderr=%s",
			validation.exitCode, validation.stdout, validation.stderr)
	}
}

func TestEntityReviseRejectsForkingAnAlreadySupersededEntity(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()
	setupDeliveredRequirement(t, dir, dbFile)

	first := runCLI(t, dir, "--db", dbFile, "entity", "revise", "REQ-001", "--reason", "First revision")
	if first.exitCode != 0 {
		t.Fatalf("first revise: exit=%d stderr=%s", first.exitCode, first.stderr)
	}

	second := runCLI(t, dir, "--db", dbFile, "entity", "revise", "REQ-001", "--reason", "Forking revision")
	if second.exitCode != 2 {
		t.Fatalf("expected exit 2 (conflict), got %d; stdout=%s stderr=%s", second.exitCode, second.stdout, second.stderr)
	}

	var resp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(second.stderr), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v\nraw: %s", err, second.stderr)
	}
	if resp.Error.Code != "CONFLICT" {
		t.Errorf("error.code = %q; want CONFLICT", resp.Error.Code)
	}
}

// setupDeliveredRequirement builds REQ-001 with an implementing interface and a
// resolved phase that delivered it, the shape a revision has to split apart.
func setupDeliveredRequirement(t *testing.T, dir, dbFile string) {
	t.Helper()
	cmds := [][]string{
		{"--db", dbFile, "entity", "add", "--type", "requirement", "--id", "REQ-001", "--title", "Query operations", "--status", "active"},
		{"--db", dbFile, "entity", "add", "--type", "interface", "--id", "API-001", "--title", "Engine.QueryScope", "--status", "active"},
		{"--db", dbFile, "entity", "add", "--type", "plan", "--id", "PLN-001", "--title", "Delivery plan", "--status", "active"},
		{"--db", dbFile, "entity", "add", "--type", "phase", "--id", "PHS-001", "--title", "Phase 1", "--status", "active"},
		{"--db", dbFile, "relation", "add", "--from", "API-001", "--to", "REQ-001", "--type", "implements"},
		{"--db", dbFile, "relation", "add", "--from", "PHS-001", "--to", "PLN-001", "--type", "belongs_to"},
		{"--db", dbFile, "relation", "add", "--from", "PHS-001", "--to", "REQ-001", "--type", "covers"},
		{"--db", dbFile, "relation", "add", "--from", "PHS-001", "--to", "REQ-001", "--type", "delivers"},
		{"--db", dbFile, "entity", "update", "PHS-001", "--status", "resolved", "--force", "--reason", "Accept for test setup"},
	}
	for _, args := range cmds {
		r := runCLI(t, dir, args...)
		if r.exitCode != 0 {
			t.Fatalf("setup failed (%v): exit=%d stderr=%s", args, r.exitCode, r.stderr)
		}
	}
}

func assertRelation(t *testing.T, relations []model.Relation, fromID string, relationType model.RelationType) {
	t.Helper()
	for _, relation := range relations {
		if relation.FromID == fromID && relation.Type == relationType {
			return
		}
	}
	t.Errorf("no %s relation from %s in %+v", relationType, fromID, relations)
}

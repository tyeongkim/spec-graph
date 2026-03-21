package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

func TestE2E_FullWorkflow(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "init", "--db", dbFile)
	if r.exitCode != 0 {
		t.Fatalf("init: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var initResp jsoncontract.InitResponse
	if err := json.Unmarshal([]byte(r.stdout), &initResp); err != nil {
		t.Fatalf("init unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !initResp.Initialized {
		t.Error("expected initialized=true")
	}

	entities := []struct {
		typ, id, title string
		extra          []string
	}{
		{"requirement", "REQ-001", "All APIs need auth", []string{"--metadata", `{"priority":"must","kind":"functional"}`}},
		{"interface", "API-001", "Auth Endpoint", []string{"--metadata", `{"kind":"http"}`}},
		{"test", "TST-001", "Auth Integration Test", []string{"--metadata", `{"kind":"integration"}`}},
		{"criterion", "ACT-001", "Login returns JWT", []string{"--metadata", `{"given":"valid creds","when":"POST /login","then":"200 + JWT"}`}},
		{"decision", "DEC-001", "Use JWT tokens", []string{"--metadata", `{"rationale":"stateless","date":"2025-01-01"}`}},
		{"phase", "PHS-001", "Phase 1 - Auth", []string{"--metadata", `{"goal":"auth system","order":1}`}},
		{"question", "QST-001", "Which OAuth provider?", nil},
		{"assumption", "ASM-001", "Single tenant only", []string{"--metadata", `{"confidence":"medium"}`}},
		{"risk", "RSK-001", "Token leak risk", nil},
		{"state", "STT-001", "User Authenticated", nil},
		{"crosscut", "XCT-001", "Audit Logging", nil},
	}

	for _, e := range entities {
		args := []string{"--db", dbFile, "entity", "add", "--type", e.typ, "--id", e.id, "--title", e.title}
		args = append(args, e.extra...)
		r := runCLI(t, dir, args...)
		if r.exitCode != 0 {
			t.Fatalf("entity add %s: exit=%d stderr=%s", e.id, r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("entity add %s unmarshal: %v", e.id, err)
		}
		if resp.Entity.ID != e.id {
			t.Errorf("entity add %s: got id=%q", e.id, resp.Entity.ID)
		}
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("entity list: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var entityList jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &entityList); err != nil {
		t.Fatalf("entity list unmarshal: %v", err)
	}
	if entityList.Count != 11 {
		t.Errorf("entity list count=%d; want 11", entityList.Count)
	}

	relations := []struct {
		from, to, relType string
	}{
		{"API-001", "REQ-001", "implements"},
		{"TST-001", "REQ-001", "verifies"},
		{"TST-001", "ACT-001", "verifies"},
		{"REQ-001", "PHS-001", "planned_in"},
		{"API-001", "PHS-001", "delivered_in"},
		{"DEC-001", "QST-001", "answers"},
		{"API-001", "STT-001", "triggers"},
		{"REQ-001", "ASM-001", "assumes"},
		{"REQ-001", "ACT-001", "has_criterion"},
		{"DEC-001", "RSK-001", "mitigates"},
		{"REQ-001", "XCT-001", "constrained_by"},
		{"REQ-001", "DEC-001", "depends_on"},
	}

	for _, rel := range relations {
		r := runCLI(t, dir, "--db", dbFile, "relation", "add",
			"--from", rel.from, "--to", rel.to, "--type", rel.relType)
		if r.exitCode != 0 {
			t.Fatalf("relation add %s->%s (%s): exit=%d stderr=%s",
				rel.from, rel.to, rel.relType, r.exitCode, r.stderr)
		}
		var resp jsoncontract.RelationResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("relation add unmarshal: %v", err)
		}
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "list")
	if r.exitCode != 0 {
		t.Fatalf("relation list: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var relList jsoncontract.RelationListResponse
	if err := json.Unmarshal([]byte(r.stdout), &relList); err != nil {
		t.Fatalf("relation list unmarshal: %v", err)
	}
	if relList.Count != 12 {
		t.Errorf("relation list count=%d; want 12", relList.Count)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "update", "REQ-001",
		"--title", "All endpoints need auth", "--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("entity update: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("entity get after update: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var getResp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &getResp); err != nil {
		t.Fatalf("entity get unmarshal: %v", err)
	}
	if getResp.Entity.Title != "All endpoints need auth" {
		t.Errorf("updated title=%q; want 'All endpoints need auth'", getResp.Entity.Title)
	}
	if getResp.Entity.Status != "active" {
		t.Errorf("updated status=%q; want active", getResp.Entity.Status)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "API-001", "--to", "REQ-001", "--type", "implements")
	if r.exitCode != 0 {
		t.Fatalf("relation delete: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "list")
	if r.exitCode != 0 {
		t.Fatalf("relation list after delete: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	if err := json.Unmarshal([]byte(r.stdout), &relList); err != nil {
		t.Fatalf("relation list unmarshal: %v", err)
	}
	if relList.Count != 11 {
		t.Errorf("relation list count after delete=%d; want 11", relList.Count)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "delete", "REQ-001")
	if r.exitCode != 3 {
		t.Fatalf("delete entity with relations: expected exit 3, got %d; stderr=%s", r.exitCode, r.stderr)
	}
	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("delete entity error unmarshal: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("delete entity with relations code=%q; want INVALID_INPUT", errResp.Error.Code)
	}

	r = runCLI(t, dir, "--db", dbFile, "relation", "delete",
		"--from", "DEC-001", "--to", "QST-001", "--type", "answers")
	if r.exitCode != 0 {
		t.Fatalf("relation delete for QST-001: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "delete", "QST-001")
	if r.exitCode != 0 {
		t.Fatalf("entity delete QST-001: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var delResp jsoncontract.DeleteResponse
	if err := json.Unmarshal([]byte(r.stdout), &delResp); err != nil {
		t.Fatalf("entity delete unmarshal: %v", err)
	}
	if delResp.Deleted != "QST-001" {
		t.Errorf("deleted=%q; want QST-001", delResp.Deleted)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("final entity list: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	if err := json.Unmarshal([]byte(r.stdout), &entityList); err != nil {
		t.Fatalf("final entity list unmarshal: %v", err)
	}
	if entityList.Count != 10 {
		t.Errorf("final entity count=%d; want 10", entityList.Count)
	}
}

func TestE2E_EdgeCases(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	t.Run("unicode_title", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "requirement", "--id", "REQ-100",
			"--title", "모든 API는 인증 필요")
		if r.exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-100")
		if r.exitCode != 0 {
			t.Fatalf("get exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Entity.Title != "모든 API는 인증 필요" {
			t.Errorf("title=%q; want '모든 API는 인증 필요'", resp.Entity.Title)
		}
	})

	t.Run("large_metadata", func(t *testing.T) {
		longVal := strings.Repeat("a", 4000)
		meta := `{"data":"` + longVal + `"}`

		r := runCLI(t, dir, "--db", dbFile, "entity", "add",
			"--type", "requirement", "--id", "REQ-101",
			"--title", "Big", "--metadata", meta)
		if r.exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.EntityResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		var parsed map[string]string
		if err := json.Unmarshal(resp.Entity.Metadata, &parsed); err != nil {
			t.Fatalf("metadata unmarshal: %v", err)
		}
		if len(parsed["data"]) != 4000 {
			t.Errorf("metadata data length=%d; want 4000", len(parsed["data"]))
		}
	})

	t.Run("empty_filtered_list", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "list", "--type", "crosscut")
		if r.exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		var resp jsoncontract.EntityListResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
		}
		if resp.Count != 0 {
			t.Errorf("count=%d; want 0", resp.Count)
		}
		if resp.Entities == nil {
			t.Error("entities should be non-nil empty slice, got nil")
		}
	})

	t.Run("all_json_parseable", func(t *testing.T) {
		r := runCLI(t, dir, "--db", dbFile, "entity", "list")
		if r.exitCode != 0 {
			t.Fatalf("exit=%d", r.exitCode)
		}
		if !json.Valid([]byte(r.stdout)) {
			t.Errorf("entity list output is not valid JSON: %s", r.stdout)
		}

		r = runCLI(t, dir, "--db", dbFile, "relation", "list")
		if r.exitCode != 0 {
			t.Fatalf("exit=%d", r.exitCode)
		}
		if !json.Valid([]byte(r.stdout)) {
			t.Errorf("relation list output is not valid JSON: %s", r.stdout)
		}
	})
}

func TestE2E_ErrorConsistency(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "interface", "--id", "API-001", "--title", "Test API")
	if r.exitCode != 0 {
		t.Fatalf("setup entity: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Test Req")
	if r.exitCode != 0 {
		t.Fatalf("setup entity: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	type errorCase struct {
		name     string
		args     []string
		wantExit int
		wantCode string
	}

	cases := []errorCase{
		{
			name:     "entity_not_found",
			args:     []string{"--db", dbFile, "entity", "get", "REQ-999"},
			wantExit: 2,
			wantCode: "ENTITY_NOT_FOUND",
		},
		{
			name:     "duplicate_entity",
			args:     []string{"--db", dbFile, "entity", "add", "--type", "interface", "--id", "API-001", "--title", "Dup"},
			wantExit: 2,
			wantCode: "DUPLICATE_ENTITY",
		},
		{
			name:     "invalid_id",
			args:     []string{"--db", dbFile, "entity", "add", "--type", "requirement", "--id", "BAD-001", "--title", "Bad"},
			wantExit: 3,
			wantCode: "INVALID_INPUT",
		},
		{
			name:     "invalid_edge",
			args:     []string{"--db", dbFile, "relation", "add", "--from", "REQ-001", "--to", "API-001", "--type", "implements"},
			wantExit: 2,
			wantCode: "INVALID_EDGE",
		},
		{
			name:     "self_loop",
			args:     []string{"--db", dbFile, "relation", "add", "--from", "REQ-001", "--to", "REQ-001", "--type", "depends_on"},
			wantExit: 2,
			wantCode: "SELF_LOOP",
		},
		{
			name:     "relation_not_found",
			args:     []string{"--db", dbFile, "relation", "delete", "--from", "API-001", "--to", "REQ-001", "--type", "implements"},
			wantExit: 2,
			wantCode: "RELATION_NOT_FOUND",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runCLI(t, dir, tc.args...)
			if r.exitCode != tc.wantExit {
				t.Fatalf("exit=%d; want %d\nstderr: %s", r.exitCode, tc.wantExit, r.stderr)
			}

			var errResp jsoncontract.ErrorResponse
			if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
				t.Fatalf("stderr not valid error JSON: %v\nraw: %s", err, r.stderr)
			}
			if errResp.Error.Code != tc.wantCode {
				t.Errorf("code=%q; want %q", errResp.Error.Code, tc.wantCode)
			}
			if errResp.Error.Message == "" {
				t.Error("error message should not be empty")
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(r.stderr), &raw); err != nil {
				t.Fatalf("raw unmarshal: %v", err)
			}
			if _, ok := raw["error"]; !ok {
				t.Error("missing top-level 'error' key")
			}
			var detail map[string]json.RawMessage
			if err := json.Unmarshal(raw["error"], &detail); err != nil {
				t.Fatalf("error detail unmarshal: %v", err)
			}
			if _, ok := detail["code"]; !ok {
				t.Error("missing 'code' in error detail")
			}
			if _, ok := detail["message"]; !ok {
				t.Error("missing 'message' in error detail")
			}
		})
	}

	t.Run("not_initialized", func(t *testing.T) {
		noDBDir := t.TempDir()
		noDBFile := noDBDir + "/nonexistent/graph.db"
		r := runCLI(t, dir, "--db", noDBFile, "entity", "list")
		if r.exitCode != 1 {
			t.Fatalf("exit=%d; want 1\nstderr: %s", r.exitCode, r.stderr)
		}

		var errResp jsoncontract.ErrorResponse
		if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
			t.Fatalf("stderr not valid error JSON: %v\nraw: %s", err, r.stderr)
		}
		if errResp.Error.Code != "NOT_INITIALIZED" {
			t.Errorf("code=%q; want NOT_INITIALIZED", errResp.Error.Code)
		}
		if errResp.Error.Message == "" {
			t.Error("error message should not be empty")
		}
	})
}

package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func TestE2E_V04_EntityReadback(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	for _, entity := range []struct {
		id       string
		typeName string
		title    string
	}{
		{"REQ-010", "requirement", "User Authentication"},
		{"DEC-010", "decision", "Use OAuth2"},
	} {
		r := runCLI(t, dir, "--root", filepath.Dir(dbFile), "entity", "add",
			"--id", entity.id, "--type", entity.typeName, "--title", entity.title)
		if r.exitCode != 0 {
			t.Fatalf("entity add %s failed: exit=%d stderr=%s", entity.id, r.exitCode, r.stderr)
		}
	}

	t.Run("entity_get_after_create", func(t *testing.T) {
		r := runCLI(t, dir, "--root", filepath.Dir(dbFile), "entity", "get", "REQ-010")
		if r.exitCode != 0 {
			t.Fatalf("entity get REQ-010 failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Entity.ID != "REQ-010" {
			t.Errorf("entity id = %q; want REQ-010", resp.Entity.ID)
		}
		if resp.Entity.Type != "requirement" {
			t.Errorf("entity type = %q; want requirement", resp.Entity.Type)
		}
		if resp.Entity.Title != "User Authentication" {
			t.Errorf("entity title = %q; want 'User Authentication'", resp.Entity.Title)
		}
	})

	t.Run("entity_list_count", func(t *testing.T) {
		r := runCLI(t, dir, "--root", filepath.Dir(dbFile), "entity", "list")
		if r.exitCode != 0 {
			t.Fatalf("entity list failed: exit=%d stderr=%s", r.exitCode, r.stderr)
		}
		var resp jsoncontract.EntityListResponse
		if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("entity count = %d; want 2", resp.Count)
		}
	})
}

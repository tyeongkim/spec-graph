package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

func TestInitWithPath(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	r := runCLI(t, dir, "init", "--path", projDir)
	if r.exitCode != 0 {
		t.Fatalf("init --path: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.InitResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Initialized {
		t.Error("expected initialized=true")
	}

	expectedDB := filepath.Join(projDir, ".spec-graph", "graph.db")
	if _, err := os.Stat(expectedDB); os.IsNotExist(err) {
		t.Errorf("expected DB at %s, but it does not exist", expectedDB)
	}
}

func TestInitWithPathCreatesDir(t *testing.T) {
	dir := t.TempDir()
	// projDir does not exist yet — init --path should create it
	projDir := filepath.Join(dir, "new-project")

	r := runCLI(t, dir, "init", "--path", projDir)
	if r.exitCode != 0 {
		t.Fatalf("init --path (new dir): exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	expectedDB := filepath.Join(projDir, ".spec-graph", "graph.db")
	if _, err := os.Stat(expectedDB); os.IsNotExist(err) {
		t.Errorf("expected DB at %s, but it does not exist", expectedDB)
	}
}

func TestInitWithoutPath(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "graph.db")

	r := runCLI(t, dir, "init", "--db", dbFile)
	if r.exitCode != 0 {
		t.Fatalf("init (no --path): exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.InitResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Initialized {
		t.Error("expected initialized=true")
	}
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		t.Errorf("expected DB at %s, but it does not exist", dbFile)
	}
}

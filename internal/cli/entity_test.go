package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "spec-graph-test-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "spec-graph")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/spec-graph/")
	cmd.Dir = projectRoot()
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build binary: %v\n", err)
		os.Exit(1)
	}

	binaryPath = bin
	os.Exit(m.Run())
}

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runCLI(t *testing.T, dir string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir

	var stdoutBuf, stderrBuf []byte
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}

	stdoutBuf, _ = os.ReadFile("/dev/stdin")
	_ = stdoutBuf

	stdoutBuf = make([]byte, 0)
	stderrBuf = make([]byte, 0)

	buf := make([]byte, 4096)
	for {
		n, err := stdoutPipe.Read(buf)
		if n > 0 {
			stdoutBuf = append(stdoutBuf, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	for {
		n, err := stderrPipe.Read(buf)
		if n > 0 {
			stderrBuf = append(stderrBuf, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("wait command: %v", err)
		}
	}

	return cliResult{
		stdout:   string(stdoutBuf),
		stderr:   string(stderrBuf),
		exitCode: exitCode,
	}
}

func initTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "graph.db")
	r := runCLI(t, dir, "init", "--db", dbFile)
	if r.exitCode != 0 {
		t.Fatalf("init failed: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	return dbFile
}

func TestInitCreatesDB(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "sub", "graph.db")

	r := runCLI(t, dir, "init", "--db", dbFile)
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.InitResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, r.stdout)
	}
	if !resp.Initialized {
		t.Error("expected initialized=true")
	}
	if resp.Path == "" {
		t.Error("expected non-empty path")
	}

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		t.Error("database file not created")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "graph.db")

	r1 := runCLI(t, dir, "init", "--db", dbFile)
	if r1.exitCode != 0 {
		t.Fatalf("first init failed: %s", r1.stderr)
	}

	r2 := runCLI(t, dir, "init", "--db", dbFile)
	if r2.exitCode != 0 {
		t.Fatalf("second init failed: %s", r2.stderr)
	}
}

func TestCommandWithoutInit(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "nonexistent", "graph.db")

	r := runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", r.exitCode)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v\nraw: %s", err, r.stderr)
	}
	if errResp.Error.Code != "NOT_INITIALIZED" {
		t.Errorf("expected code NOT_INITIALIZED, got %q", errResp.Error.Code)
	}
}

func TestEntityAdd(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Test Requirement")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if resp.Entity.ID != "REQ-001" {
		t.Errorf("id = %q; want REQ-001", resp.Entity.ID)
	}
	if resp.Entity.Type != model.EntityTypeRequirement {
		t.Errorf("type = %q; want requirement", resp.Entity.Type)
	}
	if resp.Entity.Status != model.EntityStatusDraft {
		t.Errorf("status = %q; want draft", resp.Entity.Status)
	}
	if resp.Entity.Title != "Test Requirement" {
		t.Errorf("title = %q; want 'Test Requirement'", resp.Entity.Title)
	}
}

func TestEntityAddWithAllFields(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Design Choice",
		"--description", "A design decision",
		"--metadata", `{"priority":"high"}`,
		"--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entity.Description != "A design decision" {
		t.Errorf("description = %q", resp.Entity.Description)
	}
	if resp.Entity.Status != model.EntityStatusActive {
		t.Errorf("status = %q; want active", resp.Entity.Status)
	}

	var meta map[string]string
	if err := json.Unmarshal(resp.Entity.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["priority"] != "high" {
		t.Errorf("metadata.priority = %q; want high", meta["priority"])
	}
}

func TestEntityAddInvalidID(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "BAD-ID", "--title", "Bad")
	if r.exitCode != 3 {
		t.Fatalf("expected exit 3, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "INVALID_INPUT" {
		t.Errorf("code = %q; want INVALID_INPUT", errResp.Error.Code)
	}
}

func TestEntityAddDuplicate(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "First")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Duplicate")
	if r.exitCode != 2 {
		t.Fatalf("expected exit 2, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "DUPLICATE_ENTITY" {
		t.Errorf("code = %q; want DUPLICATE_ENTITY", errResp.Error.Code)
	}
}

func TestEntityAddMissingFlags(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add")
	if r.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required flags")
	}
}

func TestEntityGet(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Get Test")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entity.ID != "REQ-001" {
		t.Errorf("id = %q; want REQ-001", resp.Entity.ID)
	}
	if resp.Entity.Title != "Get Test" {
		t.Errorf("title = %q; want 'Get Test'", resp.Entity.Title)
	}
}

func TestEntityGetNotFound(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "get", "REQ-999")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", r.exitCode)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code = %q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

func TestEntityListAll(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "First")
	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Second")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d; want 2", resp.Count)
	}
	if len(resp.Entities) != 2 {
		t.Errorf("len(entities) = %d; want 2", len(resp.Entities))
	}
}

func TestEntityListWithTypeFilter(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req")
	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "decision", "--id", "DEC-001", "--title", "Dec")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "list", "--type", "requirement")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("count = %d; want 1", resp.Count)
	}
	if resp.Entities[0].Type != model.EntityTypeRequirement {
		t.Errorf("type = %q; want requirement", resp.Entities[0].Type)
	}
}

func TestEntityListWithStatusFilter(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Draft")
	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-002", "--title", "Active", "--status", "active")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "list", "--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("count = %d; want 1", resp.Count)
	}
}

func TestEntityListEmpty(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d; want 0", resp.Count)
	}
	if resp.Entities == nil {
		t.Error("entities should be non-nil empty slice")
	}
}

func TestEntityUpdateTitle(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Original")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "update", "REQ-001",
		"--title", "Updated Title")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entity.Title != "Updated Title" {
		t.Errorf("title = %q; want 'Updated Title'", resp.Entity.Title)
	}
}

func TestEntityUpdateStatus(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Test")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "update", "REQ-001",
		"--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entity.Status != model.EntityStatusActive {
		t.Errorf("status = %q; want active", resp.Entity.Status)
	}
}

func TestEntityUpdateMetadata(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Test")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "update", "REQ-001",
		"--metadata", `{"version":2}`)
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(resp.Entity.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["version"] != float64(2) {
		t.Errorf("metadata.version = %v; want 2", meta["version"])
	}
}

func TestEntityUpdateNotFound(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "update", "REQ-999",
		"--title", "Nope")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", r.exitCode)
	}
}

func TestEntityDeprecate(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "To Deprecate")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "deprecate", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.EntityResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entity.Status != model.EntityStatusDeprecated {
		t.Errorf("status = %q; want deprecated", resp.Entity.Status)
	}
}

func TestEntityDelete(t *testing.T) {
	dbFile := initTestProject(t)

	runCLI(t, t.TempDir(), "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "To Delete")

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "delete", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("exit %d; stderr: %s", r.exitCode, r.stderr)
	}

	var resp jsoncontract.DeleteResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Deleted != "REQ-001" {
		t.Errorf("deleted = %q; want REQ-001", resp.Deleted)
	}
}

func TestEntityDeleteNotFound(t *testing.T) {
	dbFile := initTestProject(t)

	r := runCLI(t, t.TempDir(), "--db", dbFile, "entity", "delete", "REQ-999")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1, got %d; stderr: %s", r.exitCode, r.stderr)
	}

	var errResp jsoncontract.ErrorResponse
	if err := json.Unmarshal([]byte(r.stderr), &errResp); err != nil {
		t.Fatalf("unmarshal stderr: %v", err)
	}
	if errResp.Error.Code != "ENTITY_NOT_FOUND" {
		t.Errorf("code = %q; want ENTITY_NOT_FOUND", errResp.Error.Code)
	}
}

func TestEntityFullLifecycle(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Lifecycle Test")
	if r.exitCode != 0 {
		t.Fatalf("add failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("get failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("list failed: %s", r.stderr)
	}
	var listResp jsoncontract.EntityListResponse
	json.Unmarshal([]byte(r.stdout), &listResp)
	if listResp.Count != 1 {
		t.Errorf("list count = %d; want 1", listResp.Count)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "update", "REQ-001",
		"--title", "Updated", "--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("update failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "deprecate", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("deprecate failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "delete", "REQ-001")
	if r.exitCode != 0 {
		t.Fatalf("delete failed: %s", r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "get", "REQ-001")
	if r.exitCode != 1 {
		t.Fatalf("expected exit 1 after delete, got %d", r.exitCode)
	}
}

package cli_test

import (
	"encoding/json"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

// startServe launches `spec-graph serve` as a long-running subprocess with an
// open stdin pipe, so the server stays alive (and, under the old lifetime-lock
// model, would hold the project lock) until the returned stop func is called.
func startServe(t *testing.T, dir, dbFile string) (stdin io.WriteCloser, stop func()) {
	t.Helper()

	cmd := exec.Command(binaryPath, "--db", dbFile, "serve")
	cmd.Dir = dir

	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("serve stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}

	// Give the server a moment to open the engine and begin serving.
	time.Sleep(300 * time.Millisecond)

	stop = func() {
		_ = in.Close()
		_ = cmd.Wait()
	}
	return in, stop
}

func TestE2E_CLICoexistsWithRunningServer(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	if r := runCLI(t, dir, "init", "--db", dbFile); r.exitCode != 0 {
		t.Fatalf("init: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	_, stop := startServe(t, dir, dbFile)
	defer stop()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add", "--type", "requirement", "--id", "REQ-001", "--title", "Concurrent with server")
	if r.exitCode != 0 {
		t.Fatalf("entity add while server running: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runCLI(t, dir, "--db", dbFile, "entity", "list")
	if r.exitCode != 0 {
		t.Fatalf("entity list while server running: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	var list jsoncontract.EntityListResponse
	if err := json.Unmarshal([]byte(r.stdout), &list); err != nil {
		t.Fatalf("entity list unmarshal: %v\nraw: %s", err, r.stdout)
	}
	if list.Count != 1 {
		t.Errorf("entity list count=%d while server running; want 1", list.Count)
	}
}

func TestE2E_DualServersCoexist(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	if r := runCLI(t, dir, "init", "--db", dbFile); r.exitCode != 0 {
		t.Fatalf("init: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	_, stop1 := startServe(t, dir, dbFile)
	defer stop1()
	_, stop2 := startServe(t, dir, dbFile)
	defer stop2()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add", "--type", "requirement", "--id", "REQ-001", "--title", "Two servers up")
	if r.exitCode != 0 {
		t.Fatalf("entity add with two servers running: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
}

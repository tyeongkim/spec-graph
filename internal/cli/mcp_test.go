package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// jsonrpcResponse is a generic JSON-RPC 2.0 response envelope.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// runMCP pipes newline-delimited JSON-RPC messages to `spec-graph mcp --db <dbFile>`
// via stdin, closes stdin, and returns all parsed JSON-RPC responses.
func runMCP(t *testing.T, dbFile string, messages ...string) []jsonrpcResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--db", dbFile, "mcp")

	var stdin bytes.Buffer
	for _, msg := range messages {
		stdin.WriteString(msg)
		stdin.WriteByte('\n')
	}
	cmd.Stdin = &stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Context deadline exceeded is expected — the MCP server runs until stdin closes
		// and may take a moment to shut down. We only fail on unexpected errors.
		if ctx.Err() == nil {
			// Not a timeout — check if we got any output (server may exit after processing).
			if stdout.Len() == 0 {
				t.Fatalf("mcp command failed with no output: %v\nstderr: %s", err, stderr.String())
			}
		}
	}

	// Parse all JSON-RPC response objects from stdout.
	var responses []jsonrpcResponse
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var resp jsonrpcResponse
		if err := dec.Decode(&resp); err != nil {
			t.Fatalf("decode JSON-RPC response: %v\nremaining stdout: %s", err, stdout.String())
		}
		// Skip notifications (no id).
		if resp.ID != nil {
			responses = append(responses, resp)
		}
	}

	return responses
}

func TestMCPInitialize(t *testing.T) {
	dbFile := initTestProject(t)

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	)

	if len(responses) < 1 {
		t.Fatalf("expected at least 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q; want 2.0", resp.JSONRPC)
	}

	var result struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools json.RawMessage `json:"tools"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal initialize result: %v\nraw: %s", err, resp.Result)
	}

	if result.ServerInfo.Name != "spec-graph" {
		t.Errorf("serverInfo.name = %q; want spec-graph", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected capabilities.tools to be present")
	}
}

func TestMCPToolsList(t *testing.T) {
	dbFile := initTestProject(t)

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	resp := responses[1]

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tools/list result: %v\nraw: %s", err, resp.Result)
	}

	advertised := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		advertised[tool.Name] = true
	}

	for _, name := range []string{
		"plan_status", "phase_brief", "phase_gate", "change_impact",
		"get_entity", "list_entities", "list_relations", "query_path",
		"apply_batch", "update_entity", "delete_entity", "delete_relation", "next_phase",
	} {
		if !advertised[name] {
			t.Errorf("tool %q is not advertised over stdio", name)
		}
	}

	// export stays a CLI and RPC surface: an agent has no use for a diagram string.
	for _, name := range []string{"export", "import_entities", "bootstrap_import"} {
		if advertised[name] {
			t.Errorf("tool %q is advertised but was excluded from the MCP surface", name)
		}
	}
}

func TestMCPToolCallPhaseBrief(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "plan", "--id", "PLN-001", "--title", "Plan", "--status", "active")
	if r.exitCode != 0 {
		t.Fatalf("add plan: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "phase", "--id", "PHS-001", "--title", "Phase 1")
	if r.exitCode != 0 {
		t.Fatalf("add phase: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Requirement 1")
	if r.exitCode != 0 {
		t.Fatalf("add req: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "PLN-001", "--type", "belongs_to")
	if r.exitCode != 0 {
		t.Fatalf("add belongs_to: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	r = runCLI(t, dir, "--db", dbFile, "relation", "add",
		"--from", "PHS-001", "--to", "REQ-001", "--type", "covers")
	if r.exitCode != 0 {
		t.Fatalf("add covers: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"phase_brief","arguments":{"phase_id":"PHS-001"}}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	result := decodeToolCall(t, responses[1])
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "REQ-001") {
		t.Errorf("brief should carry the covered REQ-001; got: %s", text)
	}
	if !strings.Contains(text, "PHS-001") {
		t.Errorf("brief should carry PHS-001; got: %s", text)
	}
}

// decodeToolCall unwraps the tools/call envelope shared by the assertions below.
func decodeToolCall(t *testing.T, resp jsonrpcResponse) struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
} {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tools/call result: %v\nraw: %s", err, resp.Result)
	}
	return result
}

func TestMCPToolCallError(t *testing.T) {
	dbFile := initTestProject(t)

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"phase_brief","arguments":{"phase_id":"PHS-999"}}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	result := decodeToolCall(t, responses[1])
	if !result.IsError {
		t.Fatalf("expected isError=true for a missing phase; got content: %+v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected error content")
	}
	if !strings.Contains(strings.ToLower(result.Content[0].Text), "not found") {
		t.Errorf("error should say the phase was not found; got: %s", result.Content[0].Text)
	}
}

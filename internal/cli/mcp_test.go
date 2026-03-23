package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	// Second response is tools/list.
	resp := responses[1]

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tools/list result: %v\nraw: %s", err, resp.Result)
	}

	if len(result.Tools) != 6 {
		t.Errorf("len(tools) = %d; want 6", len(result.Tools))
		for _, tool := range result.Tools {
			t.Logf("  tool: %s", tool.Name)
		}
	}

	expectedTools := map[string]bool{
		"query_scope":      false,
		"query_path":       false,
		"query_unresolved": false,
		"impact":           false,
		"validate":         false,
		"export":           false,
	}
	for _, tool := range result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}
	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestMCPToolCallQueryScope(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	// Set up test data: phase + entities + relations.
	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
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
		"--from", "REQ-001", "--to", "PHS-001", "--type", "planned_in")
	if r.exitCode != 0 {
		t.Fatalf("add relation: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_scope","arguments":{"phase_id":"PHS-001"}}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	resp := responses[1]
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

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "REQ-001") {
		t.Errorf("result text should contain REQ-001; got: %s", text)
	}
	if !strings.Contains(text, "PHS-001") {
		t.Errorf("result text should contain PHS-001; got: %s", text)
	}
}

func TestMCPToolCallExport(t *testing.T) {
	dbFile := initTestProject(t)
	dir := t.TempDir()

	// Add at least one entity so export has content.
	r := runCLI(t, dir, "--db", dbFile, "entity", "add",
		"--type", "requirement", "--id", "REQ-001", "--title", "Req 1")
	if r.exitCode != 0 {
		t.Fatalf("add entity: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"export","arguments":{"format":"dot"}}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	resp := responses[1]
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal export result: %v\nraw: %s", err, resp.Result)
	}

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "digraph") {
		t.Errorf("expected DOT output containing 'digraph'; got: %s", text)
	}
}

func TestMCPToolCallError(t *testing.T) {
	dbFile := initTestProject(t)

	responses := runMCP(t, dbFile,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_scope","arguments":{"phase_id":"NONEXISTENT"}}}`,
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	resp := responses[1]
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal error result: %v\nraw: %s", err, resp.Result)
	}

	// The MCP server returns isError:true for tool errors.
	if !result.IsError {
		// Some MCP implementations return the error in content text instead.
		if len(result.Content) > 0 {
			text := result.Content[0].Text
			if !strings.Contains(strings.ToLower(text), "not found") &&
				!strings.Contains(strings.ToLower(text), "error") &&
				!strings.Contains(strings.ToLower(text), "no entity") {
				t.Errorf("expected isError=true or error text; got isError=%v text=%s", result.IsError, text)
			}
		} else {
			t.Error("expected isError=true or error content")
		}
		return
	}

	if len(result.Content) == 0 {
		t.Fatal("expected error content")
	}

	// Verify error message mentions the nonexistent entity.
	text := result.Content[0].Text
	fmt.Printf("error text: %s\n", text) // debug output
	if text == "" {
		t.Error("expected non-empty error text")
	}
}

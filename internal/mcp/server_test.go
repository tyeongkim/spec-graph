package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func newTestEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func seedGraph(t *testing.T, eng *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()

	entities := []specgraph.CreateEntityRequest{
		{Type: "plan", ID: "PLN-001", Title: "Plan"},
		{Type: "phase", ID: "PHS-001", Title: "Phase"},
		{Type: "requirement", ID: "REQ-001", Title: "Req"},
		{Type: "interface", ID: "API-001", Title: "Api"},
		{Type: "question", ID: "QST-001", Title: "Open question"},
	}
	for _, e := range entities {
		if _, err := eng.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create %s: %v", e.ID, err)
		}
	}

	relations := []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: "PHS-001", To: "REQ-001", Type: "covers"},
		{From: "API-001", To: "REQ-001", Type: "implements"},
	}
	for _, r := range relations {
		if _, err := eng.AddRelation(ctx, r); err != nil {
			t.Fatalf("add relation %s->%s: %v", r.From, r.To, err)
		}
	}
}

func callRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is not TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func TestNewSpecGraphServer(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	if s := NewSpecGraphServer(eng); s == nil {
		t.Fatal("NewSpecGraphServer returned nil")
	}
}

func TestHandleQueryScope(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleQueryScope(eng)(context.Background(), callRequest(map[string]any{
		"phase_id": "PHS-001",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "PHS-001") || !strings.Contains(text, "REQ-001") {
		t.Errorf("scope result missing expected entities: %s", text)
	}
}

func TestHandleQueryScopeInvalidLayer(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleQueryScope(eng)(context.Background(), callRequest(map[string]any{
		"phase_id": "PHS-001",
		"layer":    "bogus",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for invalid layer")
	}
	if !strings.Contains(resultText(t, res), "invalid layer") {
		t.Errorf("expected invalid layer message, got: %s", resultText(t, res))
	}
}

func TestHandleQueryUnresolvedHappyPath(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleQueryUnresolved(eng)(context.Background(), callRequest(map[string]any{
		"type": "question",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "QST-001") {
		t.Errorf("expected QST-001 in unresolved result: %s", resultText(t, res))
	}
}

func TestHandleQueryUnresolvedInvalidType(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)

	res, err := handleQueryUnresolved(eng)(context.Background(), callRequest(map[string]any{
		"type": "bogus",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for invalid type")
	}
	if !strings.Contains(resultText(t, res), "must be question, assumption, or risk") {
		t.Errorf("expected friendly type message, got: %s", resultText(t, res))
	}
}

func TestHandleImpactHappyPath(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleImpact(eng)(context.Background(), callRequest(map[string]any{
		"sources": "REQ-001",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "REQ-001") {
		t.Errorf("expected REQ-001 in impact result: %s", resultText(t, res))
	}
}

func TestHandleImpactInvalidDimension(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleImpact(eng)(context.Background(), callRequest(map[string]any{
		"sources":   "REQ-001",
		"dimension": "bogus",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for invalid dimension")
	}
	if !strings.Contains(resultText(t, res), "must be structural, behavioral, or planning") {
		t.Errorf("expected friendly dimension message, got: %s", resultText(t, res))
	}
}

func TestHandleImpactInvalidMinSeverity(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleImpact(eng)(context.Background(), callRequest(map[string]any{
		"sources":      "REQ-001",
		"min_severity": "bogus",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for invalid min_severity")
	}
	if !strings.Contains(resultText(t, res), "must be high, medium, or low") {
		t.Errorf("expected friendly min_severity message, got: %s", resultText(t, res))
	}
}

func TestHandleValidateHappyPath(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleValidate(eng)(context.Background(), callRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if strings.TrimSpace(resultText(t, res)) == "" {
		t.Error("expected non-empty validate result")
	}
}

func TestHandleExportDOT(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleExport(eng)(context.Background(), callRequest(map[string]any{
		"format": "dot",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "digraph") {
		t.Errorf("expected DOT output to contain 'digraph', got: %s", text)
	}
}

func TestHandleQueryPathHappyPath(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(t)
	seedGraph(t, eng)

	res, err := handleQueryPath(eng)(context.Background(), callRequest(map[string]any{
		"from_id": "API-001",
		"to_id":   "REQ-001",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "REQ-001") {
		t.Errorf("expected REQ-001 in path result: %s", resultText(t, res))
	}
}

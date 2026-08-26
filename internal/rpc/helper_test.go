package rpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// newTestEngine opens an Engine rooted at a fresh, initialized temp directory
// and registers a cleanup that closes it.
func newTestEngine(t *testing.T) *specgraph.Engine {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func callRPC(t *testing.T, dispatcher *Dispatcher, method string, params any) response {
	t.Helper()
	request, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{
		JSONRPC: jsonRPCVersion,
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		t.Fatalf("marshal %s request: %v", method, err)
	}

	payload, notification := dispatcher.Handle(context.Background(), request)
	if notification {
		t.Fatalf("%s returned a notification", method)
	}

	var envelope response
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal %s response: %v", method, err)
	}
	return envelope
}

func decodeRPCResult[T any](t *testing.T, envelope response) T {
	t.Helper()
	if envelope.Error != nil {
		t.Fatalf("RPC error: %+v", envelope.Error)
	}

	var result T
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("unmarshal RPC result: %v", err)
	}
	return result
}

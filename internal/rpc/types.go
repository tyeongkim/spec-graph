// Package rpc implements a JSON-RPC 2.0 server that exposes the full
// specgraph.Engine API (reads and writes) over a stream transport such as
// stdio. It is distinct from the read-only MCP server in internal/mcp.
package rpc

import "encoding/json"

// jsonRPCVersion is the protocol version string required on every request and
// response by the JSON-RPC 2.0 specification.
const jsonRPCVersion = "2.0"

// Standard JSON-RPC 2.0 error codes as defined by the specification.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// request is a decoded JSON-RPC 2.0 request. A request with a nil ID is a
// notification: it is processed but yields no response.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is a JSON-RPC 2.0 response. Exactly one of Result or Error is set.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the error object carried by a failed JSON-RPC response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// errorData carries the specgraph-specific failure detail in the error object's
// data field so clients can branch on the original engine error category.
type errorData struct {
	Code     string `json:"code"`
	ExitCode int    `json:"exit_code"`
}

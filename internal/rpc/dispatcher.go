package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// handlerFunc executes a single JSON-RPC method against the engine. It returns
// the result value to be marshalled into the response, or a non-nil *rpcError
// describing the failure.
type handlerFunc func(ctx context.Context, params json.RawMessage) (any, *rpcError)

// Dispatcher routes JSON-RPC method names to engine operations. It is safe for
// concurrent use: the engine it wraps is itself goroutine-safe, and the method
// table is read-only after construction.
type Dispatcher struct {
	engine  *specgraph.Engine
	methods map[string]handlerFunc
}

// NewDispatcher builds a Dispatcher that exposes every engine operation under a
// dotted method name (e.g. "entity.create", "query.scope").
func NewDispatcher(engine *specgraph.Engine) *Dispatcher {
	d := &Dispatcher{engine: engine}
	d.methods = map[string]handlerFunc{
		"entity.create":    d.entityCreate,
		"entity.get":       d.entityGet,
		"entity.list":      d.entityList,
		"entity.update":    d.entityUpdate,
		"entity.deprecate": d.entityDeprecate,
		"entity.delete":    d.entityDelete,
		"relation.add":     d.relationAdd,
		"relation.list":    d.relationList,
		"relation.delete":  d.relationDelete,
		"query.scope":      d.queryScope,
		"query.neighbors":  d.queryNeighbors,
		"query.path":       d.queryPath,
		"query.unresolved": d.queryUnresolved,
		"impact":           d.impact,
		"validate":         d.validate,
		"export":           d.export,
		"phase.next":       d.phaseNext,
		"bootstrap.import": d.bootstrapImport,
	}
	return d
}

// Handle processes a single raw JSON-RPC message. It returns the response bytes
// to write and whether the message was a notification. Notifications and
// successfully-handled requests both return nil error from this method; the
// JSON-RPC error, if any, is encoded inside the returned response bytes. When
// isNotification is true the returned bytes are nil and nothing should be
// written to the client.
func (d *Dispatcher) Handle(ctx context.Context, raw []byte) (respBytes []byte, isNotification bool) {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return d.marshalResponse(errorResponse(nil, codeParseError, "parse error", nil)), false
	}

	notification := len(req.ID) == 0

	if req.JSONRPC != jsonRPCVersion || req.Method == "" {
		if notification {
			return nil, true
		}
		return d.marshalResponse(errorResponse(req.ID, codeInvalidRequest, "invalid request", nil)), false
	}

	handler, ok := d.methods[req.Method]
	if !ok {
		if notification {
			return nil, true
		}
		return d.marshalResponse(errorResponse(req.ID, codeMethodNotFound, fmt.Sprintf("method %q not found", req.Method), nil)), false
	}

	result, rpcErr := handler(ctx, req.Params)

	if notification {
		return nil, true
	}

	if rpcErr != nil {
		return d.marshalResponse(response{JSONRPC: jsonRPCVersion, ID: req.ID, Error: rpcErr}), false
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return d.marshalResponse(errorResponse(req.ID, codeInternalError, "marshal result: "+err.Error(), nil)), false
	}
	return d.marshalResponse(response{JSONRPC: jsonRPCVersion, ID: req.ID, Result: resultBytes}), false
}

// marshalResponse encodes a response, falling back to a minimal internal-error
// envelope if encoding somehow fails.
func (d *Dispatcher) marshalResponse(resp response) []byte {
	b, err := json.Marshal(resp)
	if err != nil {
		return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"failed to marshal response"}}`)
	}
	return b
}

// errorResponse builds an error response with no specgraph-specific data.
func errorResponse(id json.RawMessage, code int, message string, data any) response {
	return response{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   &rpcError{Code: code, Message: message, Data: data},
	}
}

// engineError maps a *specgraph.Error to an rpcError. Client-caused failures
// (invalid input, conflict, validation, gate) map to -32602 (invalid params);
// runtime and unrecognized failures map to -32603 (internal error). The
// original specgraph code and process exit code are preserved in the data
// field so clients can branch on them.
func engineError(err error) *rpcError {
	var sgErr *specgraph.Error
	if !errors.As(err, &sgErr) {
		return &rpcError{Code: codeInternalError, Message: err.Error()}
	}

	code := codeInternalError
	switch sgErr.Code {
	case specgraph.CodeInvalidInput,
		specgraph.CodeConflict,
		specgraph.CodeValidationFailed,
		specgraph.CodeGateBlocked,
		specgraph.CodeNotFound:
		code = codeInvalidParams
	case specgraph.CodeRuntime:
		code = codeInternalError
	}

	return &rpcError{
		Code:    code,
		Message: sgErr.Message,
		Data:    errorData{Code: string(sgErr.Code), ExitCode: sgErr.ExitCode()},
	}
}

// decodeParams unmarshals JSON-RPC params into dst, returning an invalid-params
// rpcError on failure. Empty params are treated as an empty object.
func decodeParams(params json.RawMessage, dst any) *rpcError {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, dst); err != nil {
		return &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	return nil
}

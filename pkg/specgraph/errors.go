// Package specgraph provides the public Engine API for operating on a
// spec-graph project. It exposes a structured error model so callers can
// programmatically inspect failures by category and derive process exit codes.
package specgraph

import "errors"

// ErrorCode classifies an Error into a coarse failure category.
type ErrorCode string

// Error codes enumerate the categories of failures the Engine API can return.
const (
	CodeInvalidInput     ErrorCode = "invalid_input"
	CodeValidationFailed ErrorCode = "validation_failed"
	CodeGateBlocked      ErrorCode = "gate_blocked"
	CodeNotFound         ErrorCode = "not_found"
	CodeConflict         ErrorCode = "conflict"
	CodeInvalidState     ErrorCode = "invalid_state"
	CodeRuntime          ErrorCode = "runtime"
)

// Error is the structured error type returned by the Engine API. It carries a
// classification code, a human-readable message, and an optional wrapped cause.
type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error satisfies the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Unwrap returns the wrapped cause, enabling errors.Is and errors.As to walk
// the error chain.
func (e *Error) Unwrap() error {
	return e.Cause
}

// ExitCode maps the error's code to a process exit code:
// NotFound→1, Conflict→2, InvalidInput→3, ValidationFailed→2, GateBlocked→2, Runtime→1.
func (e *Error) ExitCode() int {
	switch e.Code {
	case CodeNotFound:
		return 1
	case CodeConflict:
		return 2
	case CodeInvalidState:
		return 2
	case CodeInvalidInput:
		return 3
	case CodeValidationFailed:
		return 2
	case CodeGateBlocked:
		return 2
	case CodeRuntime:
		return 1
	default:
		return 1
	}
}

// newError constructs an *Error with the given code, message, and optional cause.
func newError(code ErrorCode, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}

// hasCode reports whether err is or wraps an *Error with the given code.
func hasCode(err error, code ErrorCode) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// IsNotFound reports whether err is or wraps a not-found Error.
func IsNotFound(err error) bool {
	return hasCode(err, CodeNotFound)
}

// IsConflict reports whether err is or wraps a conflict Error.
func IsConflict(err error) bool {
	return hasCode(err, CodeConflict)
}

// IsInvalidInput reports whether err is or wraps an invalid-input Error.
func IsInvalidInput(err error) bool {
	return hasCode(err, CodeInvalidInput)
}

// IsValidationFailed reports whether err is or wraps a validation-failed Error.
func IsValidationFailed(err error) bool {
	return hasCode(err, CodeValidationFailed)
}

// IsGateBlocked reports whether err is or wraps a gate-blocked Error.
func IsGateBlocked(err error) bool {
	return hasCode(err, CodeGateBlocked)
}

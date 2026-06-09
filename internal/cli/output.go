package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// exitError carries a process exit code; a nil err means stdout already holds
// the payload and nothing extra should be written to stderr.
type exitError struct {
	err  error
	code int
}

func (e *exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit %d", e.code)
}

func (e *exitError) Unwrap() error { return e.err }

func writeJSON(cmd *cobra.Command, data any) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return writeError(cmd, fmt.Errorf("marshal response: %w", err), 1)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func writeError(cmd *cobra.Command, err error, exitCode int) error {
	resp := jsoncontract.ErrorResponse{
		Error: jsoncontract.ErrorDetail{
			Code:    errorCode(err),
			Message: err.Error(),
		},
	}
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(cmd.ErrOrStderr(), string(out))
	return &exitError{err: err, code: exitCode}
}

func handleError(cmd *cobra.Command, err error) error {
	c, _ := classifyError(err)
	return writeError(cmd, err, c.exit)
}

func errorCode(err error) string {
	c, _ := classifyError(err)
	return c.code
}

// errorClass maps a model error to its JSON code and process exit code.
type errorClass struct {
	code string
	exit int
}

// classifyError returns the code+exit for a known model error, using errors.As
// so wrapped errors are matched. The bool is false for unknown errors.
func classifyError(err error) (errorClass, bool) {
	var (
		entityNotFound    *model.ErrEntityNotFound
		duplicateEntity   *model.ErrDuplicateEntity
		invalidInput      *model.ErrInvalidInput
		notInitialized    *model.ErrNotInitialized
		invalidEdge       *model.ErrInvalidEdge
		selfLoop          *model.ErrSelfLoop
		duplicateRelation *model.ErrDuplicateRelation
		relationNotFound  *model.ErrRelationNotFound
		changesetNotFound *model.ErrChangesetNotFound
	)
	switch {
	case errors.As(err, &entityNotFound):
		return errorClass{"ENTITY_NOT_FOUND", 1}, true
	case errors.As(err, &duplicateEntity):
		return errorClass{"DUPLICATE_ENTITY", 2}, true
	case errors.As(err, &invalidInput):
		return errorClass{"INVALID_INPUT", 3}, true
	case errors.As(err, &notInitialized):
		return errorClass{"NOT_INITIALIZED", 1}, true
	case errors.As(err, &invalidEdge):
		return errorClass{"INVALID_EDGE", 3}, true
	case errors.As(err, &selfLoop):
		return errorClass{"SELF_LOOP", 3}, true
	case errors.As(err, &duplicateRelation):
		return errorClass{"DUPLICATE_RELATION", 2}, true
	case errors.As(err, &relationNotFound):
		return errorClass{"RELATION_NOT_FOUND", 1}, true
	case errors.As(err, &changesetNotFound):
		return errorClass{"CHANGESET_NOT_FOUND", 1}, true
	default:
		return errorClass{"INTERNAL_ERROR", 1}, false
	}
}

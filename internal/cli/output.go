package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
)

func writeJSON(cmd *cobra.Command, data any) {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		writeError(cmd, fmt.Errorf("marshal response: %w", err), 1)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
}

func writeError(cmd *cobra.Command, err error, exitCode int) {
	resp := jsoncontract.ErrorResponse{
		Error: jsoncontract.ErrorDetail{
			Code:    errorCode(err),
			Message: err.Error(),
		},
	}
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(cmd.ErrOrStderr(), string(out))
	os.Exit(exitCode)
}

func handleError(cmd *cobra.Command, err error) {
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
		writeError(cmd, err, 1)
	case errors.As(err, &duplicateEntity):
		writeError(cmd, err, 2)
	case errors.As(err, &invalidInput):
		writeError(cmd, err, 3)
	case errors.As(err, &notInitialized):
		writeError(cmd, err, 1)
	case errors.As(err, &invalidEdge):
		writeError(cmd, err, 3)
	case errors.As(err, &selfLoop):
		writeError(cmd, err, 3)
	case errors.As(err, &duplicateRelation):
		writeError(cmd, err, 2)
	case errors.As(err, &relationNotFound):
		writeError(cmd, err, 1)
	case errors.As(err, &changesetNotFound):
		writeError(cmd, err, 1)
	default:
		writeError(cmd, err, 1)
	}
}

func errorCode(err error) string {
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
		return "ENTITY_NOT_FOUND"
	case errors.As(err, &duplicateEntity):
		return "DUPLICATE_ENTITY"
	case errors.As(err, &invalidInput):
		return "INVALID_INPUT"
	case errors.As(err, &notInitialized):
		return "NOT_INITIALIZED"
	case errors.As(err, &invalidEdge):
		return "INVALID_EDGE"
	case errors.As(err, &selfLoop):
		return "SELF_LOOP"
	case errors.As(err, &duplicateRelation):
		return "DUPLICATE_RELATION"
	case errors.As(err, &relationNotFound):
		return "RELATION_NOT_FOUND"
	case errors.As(err, &changesetNotFound):
		return "CHANGESET_NOT_FOUND"
	default:
		return "INTERNAL_ERROR"
	}
}

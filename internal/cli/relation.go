package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// engineErr maps a structured *specgraph.Error back to the model.Err* type that
// handleError recognizes, so engine failures resolve to the correct exit code
// and JSON error code. Errors already wrapping a model.Err* (invalid-edge,
// duplicate, relation-not-found) pass through untouched.
func engineErr(err error, from, to string) error {
	var se *specgraph.Error
	if !errors.As(err, &se) {
		return err
	}
	if se.Cause != nil {
		return err
	}

	switch {
	case se.Code == specgraph.CodeNotFound:
		id := from
		if to != "" && strings.Contains(se.Message, to) {
			id = to
		}
		return &model.ErrEntityNotFound{ID: id}
	case se.Code == specgraph.CodeInvalidInput && from != "" && from == to:
		return &model.ErrSelfLoop{ID: from}
	case se.Code == specgraph.CodeInvalidInput:
		return &model.ErrInvalidInput{Message: se.Message}
	default:
		return err
	}
}

var relationCmd = &cobra.Command{
	Use:   "relation",
	Short: "Manage relations",
}

var relationAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new relation",
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		relType, _ := cmd.Flags().GetString("type")
		weight, _ := cmd.Flags().GetFloat64("weight")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		if from == "" || to == "" || relType == "" {
			return handleError(cmd, &model.ErrInvalidInput{Message: "flags --from, --to, and --type are required"})
		}

		var metadata json.RawMessage
		if metadataStr != "" {
			if !json.Valid([]byte(metadataStr)) {
				return handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			metadata = json.RawMessage(metadataStr)
		}

		rel, err := engine.AddRelation(cmd.Context(), specgraph.AddRelationRequest{
			From:     from,
			To:       to,
			Type:     relType,
			Weight:   weight,
			Metadata: metadata,
		})
		if err != nil {
			return handleError(cmd, engineErr(err, from, to))
		}

		return writeJSON(cmd, jsoncontract.RelationResponse{Relation: rel})
	},
}

var relationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List relations",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromFilter, _ := cmd.Flags().GetString("from")
		toFilter, _ := cmd.Flags().GetString("to")
		typeFilter, _ := cmd.Flags().GetString("type")

		layerStr, err := ParseLayerFlagString(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		relations, count, err := engine.ListRelations(cmd.Context(), specgraph.ListRelationsRequest{
			From:  fromFilter,
			To:    toFilter,
			Type:  typeFilter,
			Layer: layerStr,
		})
		if err != nil {
			return handleError(cmd, err)
		}

		return writeJSON(cmd, jsoncontract.RelationListResponse{Relations: relations, Count: count})
	},
}

var relationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a relation",
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		relType, _ := cmd.Flags().GetString("type")

		if from == "" || to == "" || relType == "" {
			return handleError(cmd, &model.ErrInvalidInput{Message: "flags --from, --to, and --type are required"})
		}

		err := engine.DeleteRelation(cmd.Context(), specgraph.DeleteRelationRequest{
			From: from,
			To:   to,
			Type: relType,
		})
		if err != nil {
			return handleError(cmd, engineErr(err, from, to))
		}

		return writeJSON(cmd, jsoncontract.DeleteResponse{Deleted: fmt.Sprintf("%s->%s[%s]", from, to, relType)})
	},
}

func isSymmetricRelation(rt model.RelationType) bool {
	return rt == model.RelationConflictsWith
}

func init() {
	relationAddCmd.Flags().String("from", "", "source entity ID (required)")
	relationAddCmd.Flags().String("to", "", "target entity ID (required)")
	relationAddCmd.Flags().String("type", "", "relation type (required)")
	relationAddCmd.Flags().Float64("weight", 1.0, "relation weight")
	relationAddCmd.Flags().String("metadata", "", "relation metadata as JSON string")

	relationListCmd.Flags().String("from", "", "filter by source entity ID")
	relationListCmd.Flags().String("to", "", "filter by target entity ID")
	relationListCmd.Flags().String("type", "", "filter by relation type")

	relationDeleteCmd.Flags().String("from", "", "source entity ID (required)")
	relationDeleteCmd.Flags().String("to", "", "target entity ID (required)")
	relationDeleteCmd.Flags().String("type", "", "relation type (required)")

	relationCmd.AddCommand(relationAddCmd)
	relationCmd.AddCommand(relationListCmd)
	relationCmd.AddCommand(relationDeleteCmd)
}

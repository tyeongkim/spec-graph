package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
)

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
			handleError(cmd, &model.ErrInvalidInput{Message: "flags --from, --to, and --type are required"})
		}

		if !isValidRelationType(model.RelationType(relType)) {
			handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown relation type %q", relType)})
		}

		var metadata json.RawMessage
		if metadataStr != "" {
			if !json.Valid([]byte(metadataStr)) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			metadata = json.RawMessage(metadataStr)
		}

		rel := model.Relation{
			FromID:   from,
			ToID:     to,
			Type:     model.RelationType(relType),
			Weight:   weight,
			Metadata: metadata,
		}

		rs := store.NewRelationStore(getDB())
		created, err := rs.Create(rel)
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.RelationResponse{Relation: created})
		return nil
	},
}

var relationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List relations",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromFilter, _ := cmd.Flags().GetString("from")
		toFilter, _ := cmd.Flags().GetString("to")
		typeFilter, _ := cmd.Flags().GetString("type")

		var filters store.RelationFilters
		if fromFilter != "" {
			filters.FromID = &fromFilter
		}
		if toFilter != "" {
			filters.ToID = &toFilter
		}
		if typeFilter != "" {
			t := model.RelationType(typeFilter)
			filters.Type = &t
		}

		rs := store.NewRelationStore(getDB())
		relations, count, err := rs.List(filters)
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.RelationListResponse{Relations: relations, Count: count})
		return nil
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
			handleError(cmd, &model.ErrInvalidInput{Message: "flags --from, --to, and --type are required"})
		}

		rs := store.NewRelationStore(getDB())
		if err := rs.Delete(from, to, model.RelationType(relType)); err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.DeleteResponse{Deleted: fmt.Sprintf("%s->%s[%s]", from, to, relType)})
		return nil
	},
}

func isValidRelationType(t model.RelationType) bool {
	switch t {
	case model.RelationImplements,
		model.RelationVerifies,
		model.RelationDependsOn,
		model.RelationConstrainedBy,
		model.RelationPlannedIn,
		model.RelationDeliveredIn,
		model.RelationTriggers,
		model.RelationAnswers,
		model.RelationAssumes,
		model.RelationHasCriterion,
		model.RelationMitigates,
		model.RelationSupersedes,
		model.RelationConflictsWith,
		model.RelationReferences:
		return true
	default:
		return false
	}
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

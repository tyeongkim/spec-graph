package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
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

		rt := model.RelationType(relType)
		if !isValidRelationType(rt) {
			handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown relation type %q", relType)})
		}

		if from == to {
			handleError(cmd, &model.ErrSelfLoop{ID: from})
		}

		fromRec, err := queryIndex.GetEntity(from)
		if err != nil {
			handleError(cmd, fmt.Errorf("lookup from entity: %w", err))
		}
		if fromRec == nil {
			handleError(cmd, &model.ErrEntityNotFound{ID: from})
		}

		toRec, err := queryIndex.GetEntity(to)
		if err != nil {
			handleError(cmd, fmt.Errorf("lookup to entity: %w", err))
		}
		if toRec == nil {
			handleError(cmd, &model.ErrEntityNotFound{ID: to})
		}

		fromType := model.EntityType(fromRec.Type)
		toType := model.EntityType(toRec.Type)
		if !model.IsEdgeAllowed(rt, fromType, toType, nil) {
			handleError(cmd, &model.ErrInvalidEdge{FromType: fromType, ToType: toType, RelationType: rt})
		}

		var relMeta map[string]any
		if metadataStr != "" {
			if !json.Valid([]byte(metadataStr)) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			if err := json.Unmarshal([]byte(metadataStr), &relMeta); err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be a JSON object"})
			}
		}

		ownerID := from
		ownerType := fromType
		targetID := to
		if isSymmetricRelation(rt) && from > to {
			ownerID = to
			ownerType = toType
			targetID = from
		}

		ef, err := tomlStore.ReadEntity(ownerID, ownerType)
		if err != nil {
			handleError(cmd, fmt.Errorf("read owner entity: %w", err))
		}

		for _, existing := range ef.Relations {
			if existing.To == targetID && existing.Type == rt {
				handleError(cmd, &model.ErrDuplicateRelation{FromID: from, ToID: to, RelationType: rt})
			}
		}

		relWeight := weight
		if relWeight == 1.0 {
			relWeight = 0
		}

		ef.Relations = append(ef.Relations, spectoml.RelationEntry{
			To:       targetID,
			Type:     rt,
			Weight:   relWeight,
			Metadata: relMeta,
		})

		if err := tomlStore.WriteEntity(ef); err != nil {
			handleError(cmd, fmt.Errorf("write entity: %w", err))
		}

		if rt == model.RelationDelivers {
			autoActivateOnDelivers(cmd, to, toType)
		}

		var metaJSON json.RawMessage
		if len(relMeta) > 0 {
			metaJSON, _ = json.Marshal(relMeta)
		}

		created := model.Relation{
			FromID:   from,
			ToID:     to,
			Type:     rt,
			Layer:    model.LayerForRelationType(rt),
			Weight:   weight,
			Metadata: metaJSON,
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

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		var filters index.RelationFilters
		if fromFilter != "" {
			filters.FromID = fromFilter
		}
		if toFilter != "" {
			filters.ToID = toFilter
		}
		if typeFilter != "" {
			filters.Type = typeFilter
		}
		if layer != nil {
			filters.Layer = string(*layer)
		}

		records, err := queryIndex.ListRelations(filters)
		if err != nil {
			handleError(cmd, fmt.Errorf("list relations: %w", err))
		}

		relations := make([]model.Relation, 0, len(records))
		for _, rec := range records {
			relations = append(relations, model.Relation{
				FromID: rec.FromID,
				ToID:   rec.ToID,
				Type:   model.RelationType(rec.Type),
				Layer:  model.Layer(rec.Layer),
				Weight: rec.Weight,
			})
		}

		writeJSON(cmd, jsoncontract.RelationListResponse{Relations: relations, Count: len(relations)})
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

		rt := model.RelationType(relType)

		ownerID := from
		targetID := to
		if isSymmetricRelation(rt) && from > to {
			ownerID = to
			targetID = from
		}

		ownerRec, err := queryIndex.GetEntity(ownerID)
		if err != nil {
			handleError(cmd, fmt.Errorf("lookup owner entity: %w", err))
		}
		if ownerRec == nil {
			handleError(cmd, &model.ErrEntityNotFound{ID: ownerID})
		}

		ownerType := model.EntityType(ownerRec.Type)
		ef, err := tomlStore.ReadEntity(ownerID, ownerType)
		if err != nil {
			handleError(cmd, fmt.Errorf("read owner entity: %w", err))
		}

		found := false
		filtered := make([]spectoml.RelationEntry, 0, len(ef.Relations))
		for _, rel := range ef.Relations {
			if rel.To == targetID && rel.Type == rt {
				found = true
				continue
			}
			filtered = append(filtered, rel)
		}

		if !found {
			handleError(cmd, &model.ErrRelationNotFound{Key: fmt.Sprintf("%s->%s[%s]", from, to, relType)})
		}

		ef.Relations = filtered

		if err := tomlStore.WriteEntity(ef); err != nil {
			handleError(cmd, fmt.Errorf("write entity: %w", err))
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
		model.RelationTriggers,
		model.RelationAnswers,
		model.RelationAssumes,
		model.RelationHasCriterion,
		model.RelationMitigates,
		model.RelationSupersedes,
		model.RelationConflictsWith,
		model.RelationReferences,
		model.RelationBelongsTo,
		model.RelationPrecedes,
		model.RelationBlocks,
		model.RelationCovers,
		model.RelationDelivers:
		return true
	default:
		return false
	}
}

func isSymmetricRelation(rt model.RelationType) bool {
	return rt == model.RelationConflictsWith
}

func autoActivateOnDelivers(cmd *cobra.Command, entityID string, entityType model.EntityType) {
	targetEF, err := tomlStore.ReadEntity(entityID, entityType)
	if err != nil {
		return
	}
	if targetEF.Status != model.EntityStatusDraft {
		return
	}

	targetEF.Status = model.EntityStatusActive
	targetEF.UpdatedAt = time.Now()
	if err := tomlStore.WriteEntity(targetEF); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to auto-activate %s: %v\n", entityID, err)
		return
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

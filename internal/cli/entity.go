package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
)

var entityCmd = &cobra.Command{
	Use:   "entity",
	Short: "Manage entities",
}

var entityAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new entity",
	RunE: func(cmd *cobra.Command, args []string) error {
		entityType, _ := cmd.Flags().GetString("type")
		id, _ := cmd.Flags().GetString("id")
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		status, _ := cmd.Flags().GetString("status")

		if entityType == "" || id == "" || title == "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "flags --type, --id, and --title are required"})
		}

		var metadata json.RawMessage
		if metadataStr != "" {
			if !json.Valid([]byte(metadataStr)) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			metadata = json.RawMessage(metadataStr)
		}

		entity := model.Entity{
			ID:          id,
			Type:        model.EntityType(entityType),
			Title:       title,
			Description: description,
			Metadata:    metadata,
		}
		if status != "" {
			entity.Status = model.EntityStatus(status)
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		created, err := es.Create(entity, "", "", "")
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.EntityResponse{Entity: created})
		return nil
	},
}

var entityGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an entity by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		entity, err := es.Get(args[0])
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
		return nil
	},
}

var entityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entities",
	RunE: func(cmd *cobra.Command, args []string) error {
		typeFilter, _ := cmd.Flags().GetString("type")
		statusFilter, _ := cmd.Flags().GetString("status")

		var filters store.EntityFilters
		if typeFilter != "" {
			t := model.EntityType(typeFilter)
			filters.Type = &t
		}
		if statusFilter != "" {
			s := model.EntityStatus(statusFilter)
			filters.Status = &s
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		entities, count, err := es.List(filters)
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.EntityListResponse{Entities: entities, Count: count})
		return nil
	},
}

var entityUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var fields store.UpdateFields

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			fields.Title = &v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			fields.Description = &v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			s := model.EntityStatus(v)
			fields.Status = &s
		}
		if cmd.Flags().Changed("metadata") {
			v, _ := cmd.Flags().GetString("metadata")
			if !json.Valid([]byte(v)) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			m := json.RawMessage(v)
			fields.Metadata = &m
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		updated, err := es.Update(args[0], fields, "", "", "", model.ActionUpdate)
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.EntityResponse{Entity: updated})
		return nil
	},
}

var entityDeprecateCmd = &cobra.Command{
	Use:   "deprecate [id]",
	Short: "Deprecate an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status := model.EntityStatusDeprecated
		fields := store.UpdateFields{Status: &status}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		updated, err := es.Update(args[0], fields, "", "", "", model.ActionDeprecate)
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.EntityResponse{Entity: updated})
		return nil
	},
}

var entityDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		if err := es.Delete(id, "", "", ""); err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.DeleteResponse{Deleted: id})
		return nil
	},
}

func init() {
	entityAddCmd.Flags().String("type", "", "entity type (required)")
	entityAddCmd.Flags().String("id", "", "entity ID (required)")
	entityAddCmd.Flags().String("title", "", "entity title (required)")
	entityAddCmd.Flags().String("description", "", "entity description")
	entityAddCmd.Flags().String("metadata", "", "entity metadata as JSON string")
	entityAddCmd.Flags().String("status", "", "entity status")

	entityListCmd.Flags().String("type", "", "filter by entity type")
	entityListCmd.Flags().String("status", "", "filter by entity status")

	entityUpdateCmd.Flags().String("title", "", "new title")
	entityUpdateCmd.Flags().String("description", "", "new description")
	entityUpdateCmd.Flags().String("status", "", "new status")
	entityUpdateCmd.Flags().String("metadata", "", "new metadata as JSON string")
	entityUpdateCmd.Flags().String("reason", "", "reason for update (not persisted in v0.1)")

	entityDeprecateCmd.Flags().String("reason", "", "reason for deprecation (not persisted in v0.1)")

	entityCmd.AddCommand(entityAddCmd)
	entityCmd.AddCommand(entityGetCmd)
	entityCmd.AddCommand(entityListCmd)
	entityCmd.AddCommand(entityUpdateCmd)
	entityCmd.AddCommand(entityDeprecateCmd)
	entityCmd.AddCommand(entityDeleteCmd)
}

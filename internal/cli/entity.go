package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/store"
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

		metadata = resolveMetadata(cmd, metadata)

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

		reason, _ := cmd.Flags().GetString("reason")
		actor, _ := cmd.Flags().GetString("actor")
		source, _ := cmd.Flags().GetString("source")

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		created, err := es.Create(entity, reason, actor, source)
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

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		var filters store.EntityFilters
		filters.Layer = layer
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

		metaFile, _ := cmd.Flags().GetString("metadata-file")
		if metaFile != "" {
			if cmd.Flags().Changed("metadata") {
				handleError(cmd, &model.ErrInvalidInput{Message: "--metadata and --metadata-file are mutually exclusive"})
			}
			data, err := os.ReadFile(metaFile)
			if err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: "read metadata file: " + err.Error()})
			}
			if !json.Valid(data) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata file must contain valid JSON"})
			}
			m := json.RawMessage(data)
			fields.Metadata = &m
		}

		reason, _ := cmd.Flags().GetString("reason")
		actor, _ := cmd.Flags().GetString("actor")
		source, _ := cmd.Flags().GetString("source")

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		updated, err := es.Update(args[0], fields, reason, actor, source, model.ActionUpdate)
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

		reason, _ := cmd.Flags().GetString("reason")
		actor, _ := cmd.Flags().GetString("actor")
		source, _ := cmd.Flags().GetString("source")

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		updated, err := es.Update(args[0], fields, reason, actor, source, model.ActionDeprecate)
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
		reason, _ := cmd.Flags().GetString("reason")
		actor, _ := cmd.Flags().GetString("actor")
		source, _ := cmd.Flags().GetString("source")
		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		if err := es.Delete(id, reason, actor, source); err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, jsoncontract.DeleteResponse{Deleted: id})
		return nil
	},
}

type entityImportInput struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Status      string          `json:"status,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

var entityImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Bulk import entities from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("input")
		if inputFile == "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "--input is required"})
		}

		data, err := os.ReadFile(inputFile)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: "read input file: " + err.Error()})
		}

		var items []entityImportInput
		if err := json.Unmarshal(data, &items); err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: "parse input file: " + err.Error()})
		}

		reason, _ := cmd.Flags().GetString("reason")
		actor, _ := cmd.Flags().GetString("actor")
		source, _ := cmd.Flags().GetString("source")

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)

		var created []string
		var skipped []jsoncontract.BootstrapSkippedItem
		var errors []jsoncontract.BootstrapErrorItem

		for _, item := range items {
			if item.ID == "" || item.Type == "" || item.Title == "" {
				errors = append(errors, jsoncontract.BootstrapErrorItem{
					ID:    item.ID,
					Error: "id, type, and title are required",
				})
				continue
			}

			entity := model.Entity{
				ID:          item.ID,
				Type:        model.EntityType(item.Type),
				Title:       item.Title,
				Description: item.Description,
				Metadata:    item.Metadata,
			}
			if item.Status != "" {
				entity.Status = model.EntityStatus(item.Status)
			}

			_, err := es.Create(entity, reason, actor, source)
			if err != nil {
				if isDuplicateError(err) {
					skipped = append(skipped, jsoncontract.BootstrapSkippedItem{
						ID:     item.ID,
						Reason: "already exists",
					})
				} else {
					errors = append(errors, jsoncontract.BootstrapErrorItem{
						ID:    item.ID,
						Error: err.Error(),
					})
				}
				continue
			}
			created = append(created, item.ID)
		}

		writeJSON(cmd, jsoncontract.BootstrapImportResponse{
			Created: created,
			Skipped: skipped,
			Errors:  errors,
		})
		return nil
	},
}

func isDuplicateError(err error) bool {
	_, ok := err.(*model.ErrDuplicateEntity)
	return ok
}

func init() {
	entityAddCmd.Flags().String("type", "", "entity type (required)")
	entityAddCmd.Flags().String("id", "", "entity ID (required)")
	entityAddCmd.Flags().String("title", "", "entity title (required)")
	entityAddCmd.Flags().String("description", "", "entity description")
	entityAddCmd.Flags().String("metadata", "", "entity metadata as JSON string")
	entityAddCmd.Flags().String("metadata-file", "", "path to JSON file containing metadata (mutually exclusive with --metadata)")
	entityAddCmd.Flags().String("status", "", "entity status")
	entityAddCmd.Flags().String("reason", "", "reason for creating this entity")
	entityAddCmd.Flags().String("actor", "", "actor performing the change")
	entityAddCmd.Flags().String("source", "", "source of the change")

	entityListCmd.Flags().String("type", "", "filter by entity type")
	entityListCmd.Flags().String("status", "", "filter by entity status")

	entityUpdateCmd.Flags().String("title", "", "new title")
	entityUpdateCmd.Flags().String("description", "", "new description")
	entityUpdateCmd.Flags().String("status", "", "new status")
	entityUpdateCmd.Flags().String("metadata", "", "new metadata as JSON string")
	entityUpdateCmd.Flags().String("metadata-file", "", "path to JSON file containing metadata (mutually exclusive with --metadata)")
	entityUpdateCmd.Flags().String("reason", "", "reason for update")
	entityUpdateCmd.Flags().String("actor", "", "actor performing the change")
	entityUpdateCmd.Flags().String("source", "", "source of the change")

	entityDeprecateCmd.Flags().String("reason", "", "reason for deprecation")
	entityDeprecateCmd.Flags().String("actor", "", "actor performing the change")
	entityDeprecateCmd.Flags().String("source", "", "source of the change")

	entityDeleteCmd.Flags().String("reason", "", "reason for deletion")
	entityDeleteCmd.Flags().String("actor", "", "actor performing the change")
	entityDeleteCmd.Flags().String("source", "", "source of the change")

	entityImportCmd.Flags().String("input", "", "path to JSON file containing entity array (required)")
	entityImportCmd.Flags().String("reason", "", "reason for import")
	entityImportCmd.Flags().String("actor", "", "actor performing the import")
	entityImportCmd.Flags().String("source", "", "source of the import")

	entityCmd.AddCommand(entityAddCmd)
	entityCmd.AddCommand(entityGetCmd)
	entityCmd.AddCommand(entityListCmd)
	entityCmd.AddCommand(entityUpdateCmd)
	entityCmd.AddCommand(entityDeprecateCmd)
	entityCmd.AddCommand(entityDeleteCmd)
	entityCmd.AddCommand(entityImportCmd)
}

func resolveMetadata(cmd *cobra.Command, inline json.RawMessage) json.RawMessage {
	metaFile, _ := cmd.Flags().GetString("metadata-file")
	if metaFile == "" {
		return inline
	}
	if len(inline) > 0 {
		handleError(cmd, &model.ErrInvalidInput{Message: "--metadata and --metadata-file are mutually exclusive"})
	}
	data, err := os.ReadFile(metaFile)
	if err != nil {
		handleError(cmd, &model.ErrInvalidInput{Message: "read metadata file: " + err.Error()})
	}
	if !json.Valid(data) {
		handleError(cmd, &model.ErrInvalidInput{Message: "metadata file must contain valid JSON"})
	}
	return json.RawMessage(data)
}

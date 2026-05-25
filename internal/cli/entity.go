package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/gate"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
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

		et := model.EntityType(entityType)
		if err := model.ValidateEntityID(id, et); err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}
		if tomlStore.EntityExists(id, et) {
			handleError(cmd, &model.ErrDuplicateEntity{ID: id})
		}

		entityStatus := model.EntityStatusDraft
		if status != "" {
			entityStatus = model.EntityStatus(status)
		}

		schema := spectoml.DefaultSchema()
		if err := schema.ValidateEntity(id, entityType, string(entityStatus)); err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		var meta map[string]any
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &meta); err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
		}

		ef := &spectoml.EntityFile{
			Schema:      1,
			ID:          id,
			Type:        et,
			Title:       title,
			Description: description,
			Status:      entityStatus,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Metadata:    meta,
		}

		if err := tomlStore.WriteEntity(ef); err != nil {
			handleError(cmd, fmt.Errorf("write entity: %w", err))
		}

		entity, err := ef.ToEntity()
		if err != nil {
			handleError(cmd, fmt.Errorf("convert entity: %w", err))
		}
		entity.Metadata = metadata

		writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
		return nil
	},
}

var entityGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an entity by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		entity, err := readEntityByID(cmd, id)
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

		var filters index.EntityFilters
		if typeFilter != "" {
			filters.Type = typeFilter
		}
		if statusFilter != "" {
			filters.Status = statusFilter
		}
		if layer != nil {
			filters.Layer = string(*layer)
		}

		records, err := queryIndex.ListEntities(filters)
		if err != nil {
			handleError(cmd, fmt.Errorf("list entities: %w", err))
		}

		entities := make([]model.Entity, 0, len(records))
		for _, rec := range records {
			et := model.EntityType(rec.Type)
			ef, err := tomlStore.ReadEntity(rec.ID, et)
			if err != nil {
				handleError(cmd, fmt.Errorf("read entity %q: %w", rec.ID, err))
			}
			e, err := ef.ToEntity()
			if err != nil {
				handleError(cmd, fmt.Errorf("convert entity %q: %w", rec.ID, err))
			}
			entities = append(entities, e)
		}

		writeJSON(cmd, jsoncontract.EntityListResponse{Entities: entities, Count: len(entities)})
		return nil
	},
}

var entityUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		ef, _, err := findEntityFile(cmd, id)
		if err != nil {
			handleError(cmd, err)
		}

		var oldStatus model.EntityStatus

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			ef.Title = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			ef.Description = v
		}
		if cmd.Flags().Changed("status") {
			oldStatus = ef.Status
			v, _ := cmd.Flags().GetString("status")
			schema := spectoml.DefaultSchema()
			if err := schema.ValidateEntity(ef.ID, string(ef.Type), v); err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			}
			ef.Status = model.EntityStatus(v)
		}
		if cmd.Flags().Changed("metadata") {
			v, _ := cmd.Flags().GetString("metadata")
			if !json.Valid([]byte(v)) {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be valid JSON"})
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(v), &meta); err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata must be a JSON object"})
			}
			ef.Metadata = meta
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
			var meta map[string]any
			if err := json.Unmarshal(data, &meta); err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: "metadata file must contain a JSON object"})
			}
			ef.Metadata = meta
		}

		// Gate enforcement
		forceFlag, _ := cmd.Flags().GetBool("force")
		if cmd.Flags().Changed("status") {
			target := gate.Target{
				EntityID:   id,
				EntityType: ef.Type,
				FromStatus: oldStatus,
				ToStatus:   ef.Status,
			}
			if policy := gate.LookupPolicy(target); policy != nil {
				efAdapter := &indexValidateEntityFetcher{idx: queryIndex}
				rfAdapter := &validateRelationAdapter{fetcher: &indexRelationFetcher{idx: queryIndex}}

				report, err := gate.Enforce(target, rfAdapter, efAdapter)
				if err != nil {
					handleError(cmd, fmt.Errorf("gate enforce: %w", err))
				}

				if report.Blocked {
					if forceFlag {
						if len(report.Warnings) > 0 || len(report.BlockingIssues) > 0 {
							allIssues := append(report.BlockingIssues, report.Warnings...)
							warningOutput := make([]jsoncontract.ValidateIssue, len(allIssues))
							for i, issue := range allIssues {
								warningOutput[i] = jsoncontract.ValidateIssue{
									Check:    issue.Check,
									Severity: string(issue.Severity),
									Entity:   issue.Entity,
									Message:  issue.Message,
								}
							}
							warningJSON, _ := json.Marshal(warningOutput)
							fmt.Fprintf(os.Stderr, "%s\n", warningJSON)
						}
					} else {
						issues := make([]jsoncontract.ValidateIssue, len(report.BlockingIssues))
						for i, issue := range report.BlockingIssues {
							issues[i] = jsoncontract.ValidateIssue{
								Check:    issue.Check,
								Severity: string(issue.Severity),
								Entity:   issue.Entity,
								Message:  issue.Message,
							}
						}
						warnings := make([]jsoncontract.ValidateIssue, len(report.Warnings))
						for i, issue := range report.Warnings {
							warnings[i] = jsoncontract.ValidateIssue{
								Check:    issue.Check,
								Severity: string(issue.Severity),
								Entity:   issue.Entity,
								Message:  issue.Message,
							}
						}
						bySeverity := make(map[string]int)
						for _, issue := range report.BlockingIssues {
							bySeverity[string(issue.Severity)]++
						}
						response := jsoncontract.EntityUpdateGateResponse{
							Blocked:    true,
							EntityID:   report.EntityID,
							EntityType: string(report.EntityType),
							FromStatus: string(report.FromStatus),
							ToStatus:   string(report.ToStatus),
							Issues:     issues,
							Warnings:   warnings,
							Summary: jsoncontract.ValidateSummary{
								TotalIssues: len(report.BlockingIssues),
								BySeverity:  bySeverity,
							},
						}
						writeJSON(cmd, response)
						os.Exit(2)
					}
				}
			}
		}

		ef.UpdatedAt = time.Now()

		if err := tomlStore.WriteEntity(ef); err != nil {
			handleError(cmd, fmt.Errorf("write entity: %w", err))
		}

		entity, err := ef.ToEntity()
		if err != nil {
			handleError(cmd, fmt.Errorf("convert entity %q: %w", id, err))
		}
		writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
		return nil
	},
}

var entityDeprecateCmd = &cobra.Command{
	Use:   "deprecate [id]",
	Short: "Deprecate an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		ef, _, err := findEntityFile(cmd, id)
		if err != nil {
			handleError(cmd, err)
		}

		ef.Status = model.EntityStatusDeprecated
		ef.UpdatedAt = time.Now()

		if err := tomlStore.WriteEntity(ef); err != nil {
			handleError(cmd, fmt.Errorf("write entity: %w", err))
		}

		entity, _ := ef.ToEntity()
		writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
		return nil
	},
}

var entityDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		relations, err := queryIndex.GetRelationsByEntity(id)
		if err != nil {
			handleError(cmd, fmt.Errorf("check relations: %w", err))
		}
		if len(relations) > 0 {
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("cannot delete entity %q: %d relation(s) reference it", id, len(relations)),
			})
		}

		_, et, err := findEntityFile(cmd, id)
		if err != nil {
			handleError(cmd, err)
		}

		if err := tomlStore.DeleteEntity(id, et); err != nil {
			handleError(cmd, fmt.Errorf("delete entity: %w", err))
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

			et := model.EntityType(item.Type)
			if tomlStore.EntityExists(item.ID, et) {
				skipped = append(skipped, jsoncontract.BootstrapSkippedItem{
					ID:     item.ID,
					Reason: "already exists",
				})
				continue
			}

			entityStatus := model.EntityStatusDraft
			if item.Status != "" {
				entityStatus = model.EntityStatus(item.Status)
			}

			var meta map[string]any
			if len(item.Metadata) > 0 {
				if err := json.Unmarshal(item.Metadata, &meta); err != nil {
					errors = append(errors, jsoncontract.BootstrapErrorItem{
						ID:    item.ID,
						Error: "invalid metadata JSON",
					})
					continue
				}
			}

			ef := &spectoml.EntityFile{
				Schema:      1,
				ID:          item.ID,
				Type:        et,
				Title:       item.Title,
				Description: item.Description,
				Status:      entityStatus,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Metadata:    meta,
			}

			if err := tomlStore.WriteEntity(ef); err != nil {
				errors = append(errors, jsoncontract.BootstrapErrorItem{
					ID:    item.ID,
					Error: err.Error(),
				})
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

func init() {
	entityAddCmd.Flags().String("type", "", "entity type (required)")
	entityAddCmd.Flags().String("id", "", "entity ID (required)")
	entityAddCmd.Flags().String("title", "", "entity title (required)")
	entityAddCmd.Flags().String("description", "", "entity description")
	entityAddCmd.Flags().String("metadata", "", "entity metadata as JSON string")
	entityAddCmd.Flags().String("metadata-file", "", "path to JSON file containing metadata (mutually exclusive with --metadata)")
	entityAddCmd.Flags().String("status", "", "entity status")

	entityListCmd.Flags().String("type", "", "filter by entity type")
	entityListCmd.Flags().String("status", "", "filter by entity status")

	entityUpdateCmd.Flags().String("title", "", "new title")
	entityUpdateCmd.Flags().String("description", "", "new description")
	entityUpdateCmd.Flags().String("status", "", "new status")
	entityUpdateCmd.Flags().String("metadata", "", "new metadata as JSON string")
	entityUpdateCmd.Flags().String("metadata-file", "", "path to JSON file containing metadata (mutually exclusive with --metadata)")
	entityUpdateCmd.Flags().Bool("force", false, "bypass gate checks")

	entityImportCmd.Flags().String("input", "", "path to JSON file containing entity array (required)")

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

func readEntityByID(cmd *cobra.Command, id string) (model.Entity, error) {
	rec, err := queryIndex.GetEntity(id)
	if err != nil {
		return model.Entity{}, fmt.Errorf("get entity %q: %w", id, err)
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}

	et := model.EntityType(rec.Type)
	ef, err := tomlStore.ReadEntity(id, et)
	if err != nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, fmt.Errorf("convert entity %q: %w", id, err)
	}
	return entity, nil
}

func findEntityFile(cmd *cobra.Command, id string) (*spectoml.EntityFile, model.EntityType, error) {
	rec, err := queryIndex.GetEntity(id)
	if err != nil {
		return nil, "", fmt.Errorf("get entity %q: %w", id, err)
	}
	if rec == nil {
		return nil, "", &model.ErrEntityNotFound{ID: id}
	}

	et := model.EntityType(rec.Type)
	ef, err := tomlStore.ReadEntity(id, et)
	if err != nil {
		return nil, "", &model.ErrEntityNotFound{ID: id}
	}

	return ef, et, nil
}

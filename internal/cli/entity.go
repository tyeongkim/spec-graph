package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
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

		var metadata json.RawMessage
		if metadataStr != "" {
			metadata = json.RawMessage(metadataStr)
		}
		metadata, err := resolveMetadata(cmd, metadata)
		if err != nil {
			return err
		}

		entity, err := engine.CreateEntity(cmd.Context(), specgraph.CreateEntityRequest{
			Type:        entityType,
			ID:          id,
			Title:       title,
			Description: description,
			Metadata:    metadata,
			Status:      status,
		})
		if err != nil {
			return handleEngineError(cmd, err, id)
		}

		return writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
	},
}

var entityGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get an entity by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		entity, err := engine.GetEntity(cmd.Context(), id)
		if err != nil {
			return handleEngineError(cmd, err, id)
		}

		return writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
	},
}

var entityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entities",
	RunE: func(cmd *cobra.Command, args []string) error {
		typeFilter, _ := cmd.Flags().GetString("type")
		statusFilter, _ := cmd.Flags().GetString("status")

		layerFilter, err := ParseLayerFlagString(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		entities, count, err := engine.ListEntities(cmd.Context(), specgraph.ListEntitiesRequest{
			Type:   typeFilter,
			Status: statusFilter,
			Layer:  layerFilter,
		})
		if err != nil {
			return handleEngineError(cmd, err, "")
		}

		return writeJSON(cmd, jsoncontract.EntityListResponse{Entities: entities, Count: count})
	},
}

var entityUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		req := specgraph.UpdateEntityRequest{ID: id}

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			req.Title = &v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = &v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			req.Status = &v
		}
		if cmd.Flags().Changed("metadata") {
			v, _ := cmd.Flags().GetString("metadata")
			raw := json.RawMessage(v)
			req.Metadata = &raw
		}

		metaFile, _ := cmd.Flags().GetString("metadata-file")
		if metaFile != "" {
			if cmd.Flags().Changed("metadata") {
				return handleError(cmd, &model.ErrInvalidInput{Message: "--metadata and --metadata-file are mutually exclusive"})
			}
			data, err := os.ReadFile(metaFile)
			if err != nil {
				return handleError(cmd, &model.ErrInvalidInput{Message: "read metadata file: " + err.Error()})
			}
			raw := json.RawMessage(data)
			req.Metadata = &raw
		}

		forceFlag, _ := cmd.Flags().GetBool("force")
		reason, _ := cmd.Flags().GetString("reason")
		req.Force = forceFlag
		req.Reason = reason

		res, err := engine.UpdateEntity(cmd.Context(), req)
		if err != nil {
			return handleEngineError(cmd, err, id)
		}

		switch res.Outcome {
		case specgraph.UpdateOutcomeBlocked:
			report := res.GateReport
			if report == nil {
				return fmt.Errorf("blocked update %q returned no gate report", id)
			}
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
				Outcome:    string(res.Outcome),
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
			if err := writeJSON(cmd, response); err != nil {
				return err
			}
			return &exitError{code: 2}
		case specgraph.UpdateOutcomeAppliedWithForce:
			report := res.GateReport
			if report == nil {
				return fmt.Errorf("forced update %q returned no gate report", id)
			}
			issueOutput := make([]jsoncontract.ValidateIssue, len(report.BlockingIssues))
			for i, issue := range report.BlockingIssues {
				issueOutput[i] = jsoncontract.ValidateIssue{
					Check:    issue.Check,
					Severity: string(issue.Severity),
					Entity:   issue.Entity,
					Message:  issue.Message,
				}
			}
			advisoryOutput := make([]jsoncontract.ValidateIssue, len(report.Warnings))
			for i, issue := range report.Warnings {
				advisoryOutput[i] = jsoncontract.ValidateIssue{
					Check:    issue.Check,
					Severity: string(issue.Severity),
					Entity:   issue.Entity,
					Message:  issue.Message,
				}
			}
			allIssues := append([]validate.ValidationIssue(nil), report.BlockingIssues...)
			allIssues = append(allIssues, report.Warnings...)
			warningOutput := make([]jsoncontract.ValidateIssue, len(allIssues))
			for i, issue := range allIssues {
				warningOutput[i] = jsoncontract.ValidateIssue{
					Check:    issue.Check,
					Severity: string(issue.Severity),
					Entity:   issue.Entity,
					Message:  issue.Message,
				}
			}
			warningJSON, marshalErr := json.Marshal(warningOutput)
			if marshalErr != nil {
				return fmt.Errorf("marshal gate warnings: %w", marshalErr)
			}
			fmt.Fprintf(os.Stderr, "%s\n", warningJSON)
			return writeJSON(cmd, jsoncontract.EntityUpdateSuccessResponse{
				Entity: res.Entity, Outcome: string(res.Outcome), Blocked: false,
				Issues: issueOutput, Warnings: advisoryOutput,
			})
		case specgraph.UpdateOutcomeApplied:
			return writeJSON(cmd, jsoncontract.EntityUpdateSuccessResponse{
				Entity: res.Entity, Outcome: string(res.Outcome), Blocked: false,
			})
		default:
			return fmt.Errorf("update %q returned unknown outcome %q", id, res.Outcome)
		}
	},
}

var entityDeprecateCmd = &cobra.Command{
	Use:   "deprecate [id]",
	Short: "Deprecate an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		reason, _ := cmd.Flags().GetString("reason")

		entity, err := engine.DeprecateEntity(cmd.Context(), id, reason)
		if err != nil {
			return handleEngineError(cmd, err, id)
		}

		return writeJSON(cmd, jsoncontract.EntityResponse{Entity: entity})
	},
}

var entityReviseCmd = &cobra.Command{
	Use:   "revise [id]",
	Short: "Supersede an arch entity with a new revision",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		reason, _ := cmd.Flags().GetString("reason")
		req := specgraph.ReviseEntityRequest{ID: id, Reason: reason}

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			req.Title = &v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = &v
		}
		if cmd.Flags().Changed("metadata") {
			v, _ := cmd.Flags().GetString("metadata")
			raw := json.RawMessage(v)
			req.Metadata = &raw
		}

		metadata, err := resolveMetadata(cmd, nil)
		if err != nil {
			return err
		}
		if len(metadata) > 0 {
			if req.Metadata != nil {
				return handleError(cmd, &model.ErrInvalidInput{Message: "--metadata and --metadata-file are mutually exclusive"})
			}
			req.Metadata = &metadata
		}

		result, err := engine.ReviseEntity(cmd.Context(), req)
		if err != nil {
			if specgraph.IsNotFound(err) {
				return handleError(cmd, &model.ErrEntityNotFound{ID: id})
			}
			return handleError(cmd, err)
		}

		return writeJSON(cmd, jsoncontract.EntityReviseResponse{
			Revision:   result.Revision,
			Superseded: result.Superseded,
			Carried:    result.CarriedRelations,
			Retained:   result.RetainedRelations,
		})
	},
}

var entityDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		if err := engine.DeleteEntity(cmd.Context(), id); err != nil {
			return handleEngineError(cmd, err, id)
		}

		return writeJSON(cmd, jsoncontract.DeleteResponse{Deleted: id})
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
			return handleError(cmd, &model.ErrInvalidInput{Message: "--input is required"})
		}

		data, err := os.ReadFile(inputFile)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: "read input file: " + err.Error()})
		}

		var items []entityImportInput
		if err := json.Unmarshal(data, &items); err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: "parse input file: " + err.Error()})
		}

		var created []string
		var skipped []jsoncontract.BootstrapSkippedItem
		var errItems []jsoncontract.BootstrapErrorItem

		for _, item := range items {
			if item.ID == "" || item.Type == "" || item.Title == "" {
				errItems = append(errItems, jsoncontract.BootstrapErrorItem{
					ID:    item.ID,
					Error: "id, type, and title are required",
				})
				continue
			}

			_, err := engine.CreateEntity(cmd.Context(), specgraph.CreateEntityRequest{
				Type:        item.Type,
				ID:          item.ID,
				Title:       item.Title,
				Description: item.Description,
				Status:      item.Status,
				Metadata:    item.Metadata,
			})
			if err != nil {
				if specgraph.IsConflict(err) {
					skipped = append(skipped, jsoncontract.BootstrapSkippedItem{
						ID:     item.ID,
						Reason: "already exists",
					})
					continue
				}
				errItems = append(errItems, jsoncontract.BootstrapErrorItem{
					ID:    item.ID,
					Error: err.Error(),
				})
				continue
			}

			created = append(created, item.ID)
		}

		return writeJSON(cmd, jsoncontract.BootstrapImportResponse{
			Created: created,
			Skipped: skipped,
			Errors:  errItems,
		})
	},
}

func init() {
	entityAddCmd.Flags().String("type", "", "entity type (required)")
	entityAddCmd.Flags().String("id", "", "explicit entity ID (auto-generated from --type when omitted; prefer omitting)")
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
	entityUpdateCmd.Flags().String("reason", "", "audit note for the change")
	entityDeprecateCmd.Flags().String("reason", "", "reason for deprecation")

	entityReviseCmd.Flags().String("title", "", "title for the new revision (carried forward when omitted)")
	entityReviseCmd.Flags().String("description", "", "description for the new revision (carried forward when omitted)")
	entityReviseCmd.Flags().String("metadata", "", "metadata for the new revision as JSON string (carried forward when omitted)")
	entityReviseCmd.Flags().String("metadata-file", "", "path to JSON file containing metadata (mutually exclusive with --metadata)")
	entityReviseCmd.Flags().String("reason", "", "why the entity was revised (required)")

	entityImportCmd.Flags().String("input", "", "path to JSON file containing entity array (required)")

	entityCmd.AddCommand(entityAddCmd)
	entityCmd.AddCommand(entityGetCmd)
	entityCmd.AddCommand(entityListCmd)
	entityCmd.AddCommand(entityUpdateCmd)
	entityCmd.AddCommand(entityDeprecateCmd)
	entityCmd.AddCommand(entityReviseCmd)
	entityCmd.AddCommand(entityDeleteCmd)
	entityCmd.AddCommand(entityImportCmd)
}

// handleEngineError translates a *specgraph.Error into the model error type that
// output.go's handleError recognizes, preserving the JSON error code and process
// exit code. The id is used to reconstruct identifying error messages for
// not-found and conflict cases. Non-engine errors pass through unchanged.
func handleEngineError(cmd *cobra.Command, err error, id string) error {
	switch {
	case specgraph.IsNotFound(err):
		return handleError(cmd, &model.ErrEntityNotFound{ID: id})
	case specgraph.IsConflict(err):
		return handleError(cmd, &model.ErrDuplicateEntity{ID: id})
	case specgraph.IsInvalidInput(err):
		return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
	default:
		return handleError(cmd, err)
	}
}

func resolveMetadata(cmd *cobra.Command, inline json.RawMessage) (json.RawMessage, error) {
	metaFile, _ := cmd.Flags().GetString("metadata-file")
	if metaFile == "" {
		return inline, nil
	}
	if len(inline) > 0 {
		return nil, handleError(cmd, &model.ErrInvalidInput{Message: "--metadata and --metadata-file are mutually exclusive"})
	}
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, handleError(cmd, &model.ErrInvalidInput{Message: "read metadata file: " + err.Error()})
	}
	if !json.Valid(data) {
		return nil, handleError(cmd, &model.ErrInvalidInput{Message: "metadata file must contain valid JSON"})
	}
	return json.RawMessage(data), nil
}

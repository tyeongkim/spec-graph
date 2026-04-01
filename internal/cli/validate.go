package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
	"github.com/taeyeong/spec-graph/internal/validate"
)

type validateEntityAdapter struct {
	store *store.EntityStore
}

func (a *validateEntityAdapter) Get(id string) (model.Entity, error) {
	return a.store.Get(id)
}

func (a *validateEntityAdapter) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	sf := store.EntityFilters{Type: filters.Type, Status: filters.Status, Layer: filters.Layer}
	entities, _, err := a.store.List(sf)
	return entities, err
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the specification graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetString("check")
		phaseFlag, _ := cmd.Flags().GetString("phase")
		entityFlag, _ := cmd.Flags().GetString("entity")

		if entityFlag != "" && phaseFlag != "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "--entity and --phase are mutually exclusive"})
			return nil
		}

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		// Q3 decision: --phase is only valid with --layer mapping or --layer all (nil).
		if phaseFlag != "" && layer != nil && *layer != model.LayerMapping {
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("--phase cannot be used with --layer %s; only --layer mapping or --layer all is allowed", *layer),
			})
			return nil
		}

		var opts validate.ValidateOptions
		opts.Layer = layer
		if checkFlag != "" {
			opts.Checks = strings.Split(checkFlag, ",")
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		rs := store.NewRelationStore(db, cs, hs)
		es := store.NewEntityStore(db, cs, hs)
		ef := &validateEntityAdapter{store: es}

		if phaseFlag != "" {
			entity, err := es.Get(phaseFlag)
			if err != nil {
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("phase %q not found", phaseFlag)})
				return nil
			}
			if entity.Type != model.EntityTypePhase {
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("entity %q is type %q, not phase", phaseFlag, entity.Type)})
				return nil
			}
			opts.Phase = &phaseFlag
		}

		if entityFlag != "" {
			if _, err := es.Get(entityFlag); err != nil {
				handleError(cmd, &model.ErrEntityNotFound{ID: entityFlag})
				return nil
			}
			opts.EntityID = entityFlag
		}

		result, err := validate.Validate(opts, rs, ef)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		issues := make([]jsoncontract.ValidateIssue, len(result.Issues))
		for i, issue := range result.Issues {
			issues[i] = jsoncontract.ValidateIssue{
				Check:    issue.Check,
				Severity: string(issue.Severity),
				Entity:   issue.Entity,
				Message:  issue.Message,
			}
		}

		bySeverity := make(map[string]int, len(result.Summary.BySeverity))
		for k, v := range result.Summary.BySeverity {
			bySeverity[string(k)] = v
		}

		response := jsoncontract.ValidateResponse{
			Valid:  result.Valid,
			Issues: issues,
			Summary: jsoncontract.ValidateSummary{
				TotalIssues: result.Summary.TotalIssues,
				BySeverity:  bySeverity,
			},
		}

		writeJSON(cmd, response)
		if !result.Valid {
			os.Exit(2)
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().String("check", "", "comma-separated check names: orphans,coverage,invalid_edges,superseded_refs,unresolved,cycles,conflicts,phase_order,single_active_plan,orphan_phases,exec_cycles,invalid_exec_edges,plan_coverage,delivery_completeness,mapping_consistency,invalid_mapping_edges,gates")
	validateCmd.Flags().String("phase", "", "restrict validation to entities in this phase (must be a phase entity ID)")
	validateCmd.Flags().String("entity", "", "restrict validation to issues for this entity ID")
	rootCmd.AddCommand(validateCmd)
}

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type validateRelationAdapter struct {
	fetcher *indexRelationFetcher
}

func (a *validateRelationAdapter) GetByEntity(entityID string) ([]model.Relation, error) {
	rels, err := a.fetcher.GetByEntity(entityID)
	if err != nil {
		return nil, err
	}
	for i, r := range rels {
		if r.Type == model.RelationSupersedes {
			rels[i].FromID, rels[i].ToID = r.ToID, r.FromID
		}
	}
	return rels, nil
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the specification graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetString("check")
		phaseFlag, _ := cmd.Flags().GetString("phase")
		entityFlag, _ := cmd.Flags().GetString("entity")
		includeReferencesFlag, _ := cmd.Flags().GetBool("include-references")

		if entityFlag != "" && phaseFlag != "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "--entity and --phase are mutually exclusive"})
			return nil
		}

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		if phaseFlag != "" && layer != nil && *layer != model.LayerMapping {
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("--phase cannot be used with --layer %s; only --layer mapping or --layer all is allowed", *layer),
			})
			return nil
		}

		var opts validate.ValidateOptions
		opts.Layer = layer
		opts.IncludeReferences = includeReferencesFlag
		if checkFlag != "" {
			opts.Checks = strings.Split(checkFlag, ",")
		}

		ef := &indexValidateEntityFetcher{idx: queryIndex}
		rf := &validateRelationAdapter{fetcher: &indexRelationFetcher{idx: queryIndex}}

		if phaseFlag != "" {
			entity, err := ef.Get(phaseFlag)
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
			if _, err := ef.Get(entityFlag); err != nil {
				handleError(cmd, &model.ErrEntityNotFound{ID: entityFlag})
				return nil
			}
			opts.EntityID = entityFlag
		}

		result, err := validate.Validate(opts, rf, ef)
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

		var satisfaction []jsoncontract.ValidatePhaseSatisfaction
		if len(result.Satisfaction) > 0 {
			satisfaction = make([]jsoncontract.ValidatePhaseSatisfaction, len(result.Satisfaction))
			for i, report := range result.Satisfaction {
				items := make([]jsoncontract.ValidateSatisfactionItem, len(report.Items))
				for j, item := range report.Items {
					items[j] = jsoncontract.ValidateSatisfactionItem{
						EntityID:         item.EntityID,
						EntityType:       string(item.EntityType),
						Status:           string(item.Status),
						Reason:           item.Reason,
						EvidenceID:       item.EvidenceID,
						EvidenceRelation: string(item.EvidenceRelation),
					}
				}
				satisfaction[i] = jsoncontract.ValidatePhaseSatisfaction{
					PhaseID:       report.PhaseID,
					Satisfied:     report.Satisfied,
					Total:         report.Total,
					AdvisoryCount: report.AdvisoryCount,
					Items:         items,
				}
			}
		}

		response := jsoncontract.ValidateResponse{
			Valid:  result.Valid,
			Issues: issues,
			Summary: jsoncontract.ValidateSummary{
				TotalIssues: result.Summary.TotalIssues,
				BySeverity:  bySeverity,
			},
			Satisfaction: satisfaction,
		}

		writeJSON(cmd, response)
		if !result.Valid {
			os.Exit(2)
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().String("check", "", "comma-separated check names: orphans,coverage,invalid_edges,superseded_refs,unresolved,cycles,conflicts,phase_order,single_active_plan,orphan_phases,exec_cycles,invalid_exec_edges,plan_coverage,delivery_completeness,mapping_consistency,invalid_mapping_edges,gates,phase_satisfaction")
	validateCmd.Flags().String("phase", "", "restrict validation to entities in this phase (must be a phase entity ID)")
	validateCmd.Flags().String("entity", "", "restrict validation to issues for this entity ID")
	validateCmd.Flags().Bool("include-references", false, "include 1-depth references neighbors in phase satisfaction closure as advisory members")
	rootCmd.AddCommand(validateCmd)
}

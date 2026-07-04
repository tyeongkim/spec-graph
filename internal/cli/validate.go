package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the specification graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetString("check")
		phaseFlag, _ := cmd.Flags().GetString("phase")
		entityFlag, _ := cmd.Flags().GetString("entity")
		includeReferencesFlag, _ := cmd.Flags().GetBool("include-references")

		if entityFlag != "" && phaseFlag != "" {
			return handleError(cmd, &model.ErrInvalidInput{Message: "--entity and --phase are mutually exclusive"})
		}

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		if phaseFlag != "" && layer != nil && *layer != model.LayerMapping {
			return handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("--phase cannot be used with --layer %s; only --layer mapping or --layer all is allowed", *layer),
			})
		}

		if phaseFlag != "" {
			entity, err := engine.GetEntity(cmd.Context(), phaseFlag)
			if err != nil {
				return handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("phase %q not found", phaseFlag)})
			}
			if entity.Type != model.EntityTypePhase {
				return handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("entity %q is type %q, not phase", phaseFlag, entity.Type)})
			}
		}

		if entityFlag != "" {
			if _, err := engine.GetEntity(cmd.Context(), entityFlag); err != nil {
				return handleError(cmd, &model.ErrEntityNotFound{ID: entityFlag})
			}
		}

		layerStr := ""
		if layer != nil {
			layerStr = string(*layer)
		}

		var checks []string
		if checkFlag != "" {
			checks = strings.Split(checkFlag, ",")
		}

		result, err := engine.Validate(cmd.Context(), specgraph.ValidateRequest{
			Checks:            checks,
			Phase:             phaseFlag,
			EntityID:          entityFlag,
			Layer:             layerStr,
			IncludeReferences: includeReferencesFlag,
		})
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
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

		if err := writeJSON(cmd, response); err != nil {
			return err
		}
		if !result.Valid {
			return &exitError{code: 2}
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

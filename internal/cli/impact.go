package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var impactCmd = &cobra.Command{
	Use:   "impact [sources...]",
	Short: "Analyze impact of changes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		followStr, _ := cmd.Flags().GetString("follow")
		minSevStr, _ := cmd.Flags().GetString("min-severity")
		dimStr, _ := cmd.Flags().GetString("dimension")

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		if minSevStr != "" {
			switch minSevStr {
			case "high", "medium", "low":
			default:
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown severity %q; must be high, medium, or low", minSevStr)})
				return nil
			}
		}

		if dimStr != "" {
			switch dimStr {
			case "structural", "behavioral", "planning":
			default:
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown dimension %q; must be structural, behavioral, or planning", dimStr)})
				return nil
			}
		}

		for _, src := range args {
			if _, err := engine.GetEntity(cmd.Context(), src); err != nil {
				handleError(cmd, err)
				return nil
			}
		}

		layerStr := ""
		if layer != nil {
			layerStr = string(*layer)
		}

		var follow []string
		if followStr != "" {
			follow = strings.Split(followStr, ",")
		}

		result, err := engine.Impact(cmd.Context(), specgraph.ImpactRequest{
			Sources:     args,
			Follow:      follow,
			MinSeverity: minSevStr,
			Dimension:   dimStr,
			Layer:       layerStr,
		})
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		response := convertImpactResult(result)
		writeJSON(cmd, response)
		return nil
	},
}

func convertImpactResult(r *graph.ImpactResult) jsoncontract.ImpactResponse {
	affected := make([]jsoncontract.ImpactAffected, 0, len(r.Affected))
	for _, a := range r.Affected {
		chain := make([]string, len(a.RelationChain))
		for i, rt := range a.RelationChain {
			chain[i] = string(rt)
		}

		affected = append(affected, jsoncontract.ImpactAffected{
			ID:            a.ID,
			Type:          string(a.Type),
			Depth:         a.Depth,
			Path:          a.Path,
			RelationChain: chain,
			Impact: jsoncontract.ImpactScores{
				Overall:    string(a.Overall),
				Structural: string(graph.ScoreToSeverity(a.Impact.Structural)),
				Behavioral: string(graph.ScoreToSeverity(a.Impact.Behavioral)),
				Planning:   string(graph.ScoreToSeverity(a.Impact.Planning)),
			},
			Reason: a.Reason,
		})
	}

	byType := make(map[string]int, len(r.Summary.ByType))
	for k, v := range r.Summary.ByType {
		byType[string(k)] = v
	}
	byImpact := make(map[string]int, len(r.Summary.ByImpact))
	for k, v := range r.Summary.ByImpact {
		byImpact[string(k)] = v
	}

	return jsoncontract.ImpactResponse{
		Sources:  r.Sources,
		Affected: affected,
		Summary: jsoncontract.ImpactSummary{
			Total:    r.Summary.Total,
			ByType:   byType,
			ByImpact: byImpact,
		},
	}
}

func init() {
	impactCmd.Flags().String("follow", "", "comma-separated relation types to follow")
	impactCmd.Flags().String("min-severity", "", "minimum severity filter (high, medium, low)")
	impactCmd.Flags().String("dimension", "", "restrict scoring to single dimension (structural, behavioral, planning)")

	rootCmd.AddCommand(impactCmd)
}

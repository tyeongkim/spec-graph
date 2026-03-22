package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/graph"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
)

type entityStoreAdapter struct {
	store *store.EntityStore
}

func (a *entityStoreAdapter) Get(id string) (model.Entity, error) {
	return a.store.Get(id)
}

func (a *entityStoreAdapter) List(filters graph.EntityListFilters) ([]model.Entity, error) {
	sf := store.EntityFilters{Type: filters.Type, Status: filters.Status}
	entities, _, err := a.store.List(sf)
	return entities, err
}

var impactCmd = &cobra.Command{
	Use:   "impact [sources...]",
	Short: "Analyze impact of changes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		followStr, _ := cmd.Flags().GetString("follow")
		minSevStr, _ := cmd.Flags().GetString("min-severity")
		dimStr, _ := cmd.Flags().GetString("dimension")

		var opts graph.ImpactOptions

		if followStr != "" {
			parts := strings.Split(followStr, ",")
			follow := make([]model.RelationType, 0, len(parts))
			for _, p := range parts {
				rt := model.RelationType(strings.TrimSpace(p))
				if !isValidRelationType(rt) {
					handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown relation type %q in --follow", rt)})
				}
				follow = append(follow, rt)
			}
			opts.Follow = follow
		}

		if minSevStr != "" {
			switch minSevStr {
			case "high", "medium", "low":
				sev := graph.Severity(minSevStr)
				opts.MinSeverity = &sev
			default:
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown severity %q; must be high, medium, or low", minSevStr)})
			}
		}

		if dimStr != "" {
			switch dimStr {
			case "structural", "behavioral", "planning":
				opts.Dimension = &dimStr
			default:
				handleError(cmd, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown dimension %q; must be structural, behavioral, or planning", dimStr)})
			}
		}

		rs := store.NewRelationStore(getDB())
		es := store.NewEntityStore(getDB())

		for _, src := range args {
			if _, err := es.Get(src); err != nil {
				handleError(cmd, err)
			}
		}

		result, err := graph.Impact(args, opts, rs, &entityStoreAdapter{store: es})
		if err != nil {
			handleError(cmd, err)
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

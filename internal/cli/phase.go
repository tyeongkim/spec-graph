package cli

import (
	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Phase lifecycle commands",
}

var phaseNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find and optionally activate the next eligible phase in the active plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		activate, _ := cmd.Flags().GetBool("activate")

		result, err := engine.PhaseNext(cmd.Context(), specgraph.PhaseNextRequest{
			Activate: activate,
		})
		if err != nil {
			// Engine reports "no active plan" as not-found, but the CLI JSON
			// contract maps all phase-selection failures to INVALID_INPUT.
			if specgraph.IsNotFound(err) || specgraph.IsInvalidInput(err) {
				return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			}
			return handleError(cmd, err)
		}

		response := jsoncontract.PhaseNextResponse{
			Phase: jsoncontract.PhaseNextDetail{
				ID:                   result.Phase.ID,
				Title:                result.Phase.Title,
				Status:               result.Phase.Status,
				Goal:                 result.Phase.Goal,
				Order:                result.Phase.Order,
				PredecessorsResolved: result.Phase.PredecessorsResolved,
				Metadata:             result.Phase.Metadata,
			},
			Scope: jsoncontract.PhaseNextScope{
				Total:     result.Scope.Total,
				Delivered: result.Scope.Delivered,
				Remaining: result.Scope.Remaining,
			},
			Activated: result.Activated,
		}

		return writeJSON(cmd, response)
	},
}

// relationsByEntityFunc returns all relations referencing the given entity. It
// is satisfied by *specgraph.Engine.RelationsByEntity, which acquires the
// engine lock, so phase scoping goes through the locked engine API rather than
// touching the index directly.
type relationsByEntityFunc func(entityID string) ([]model.Relation, error)

func phaseEntityScope(phaseID string, relationsByEntity relationsByEntityFunc) (map[string]bool, error) {
	rels, err := relationsByEntity(phaseID)
	if err != nil {
		return nil, err
	}
	scope := make(map[string]bool)
	for _, r := range rels {
		if r.FromID == phaseID && (r.Type == model.RelationCovers || r.Type == model.RelationDelivers) {
			scope[r.ToID] = true
		}
	}

	return scope, nil
}

func init() {
	phaseNextCmd.Flags().Bool("activate", false, "automatically transition the phase from draft to active")
	phaseCmd.AddCommand(phaseNextCmd)
}

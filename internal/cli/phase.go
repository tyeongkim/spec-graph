package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Phase lifecycle commands",
}

type PhaseNextResponse struct {
	Phase     PhaseNextDetail `json:"phase"`
	Scope     PhaseNextScope  `json:"scope"`
	Activated bool            `json:"activated"`
}

type PhaseNextDetail struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Status               string          `json:"status"`
	Goal                 string          `json:"goal"`
	Order                float64         `json:"order"`
	PredecessorsResolved bool            `json:"predecessors_resolved"`
	Metadata             json.RawMessage `json:"metadata"`
}

type PhaseNextScope struct {
	Total     int      `json:"total"`
	Delivered int      `json:"delivered"`
	Remaining []string `json:"remaining"`
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
				handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			}
			handleError(cmd, err)
		}

		response := PhaseNextResponse{
			Phase: PhaseNextDetail{
				ID:                   result.Phase.ID,
				Title:                result.Phase.Title,
				Status:               result.Phase.Status,
				Goal:                 result.Phase.Goal,
				Order:                result.Phase.Order,
				PredecessorsResolved: result.Phase.PredecessorsResolved,
				Metadata:             result.Phase.Metadata,
			},
			Scope: PhaseNextScope{
				Total:     result.Scope.Total,
				Delivered: result.Scope.Delivered,
				Remaining: result.Scope.Remaining,
			},
			Activated: result.Activated,
		}

		writeJSON(cmd, response)
		return nil
	},
}

func phaseEntityScope(phaseID string, rf *indexRelationFetcher) (map[string]bool, error) {
	rels, err := rf.GetByEntity(phaseID)
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

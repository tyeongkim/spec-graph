package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/graph"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the specification graph",
}

var queryScopeCmd = &cobra.Command{
	Use:   "scope <phase-id>",
	Short: "List entities and relations belonging to a phase",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phaseID := args[0]

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		rs := store.NewRelationStore(db, cs, hs)
		es := store.NewEntityStore(db, cs, hs)

		opts := graph.QueryScopeOptions{PhaseID: phaseID}
		result, err := graph.QueryScope(opts, rs, &entityStoreAdapter{store: es})
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		response := convertQueryScopeResult(result)
		writeJSON(cmd, response)
		return nil
	},
}

func convertQueryScopeResult(r *graph.QueryScopeResult) jsoncontract.QueryScopeResponse {
	entities := make([]jsoncontract.EntitySummary, 0, len(r.Entities))
	for _, e := range r.Entities {
		entities = append(entities, jsoncontract.EntitySummary{
			ID:     e.ID,
			Type:   string(e.Type),
			Title:  e.Title,
			Status: string(e.Status),
		})
	}

	relations := make([]jsoncontract.RelationSummary, 0, len(r.Relations))
	for _, rel := range r.Relations {
		relations = append(relations, jsoncontract.RelationSummary{
			FromID: rel.FromID,
			ToID:   rel.ToID,
			Type:   string(rel.Type),
		})
	}

	byType := make(map[string]int)
	for _, e := range r.Entities {
		byType[string(e.Type)]++
	}

	return jsoncontract.QueryScopeResponse{
		PhaseID:   r.PhaseID,
		Entities:  entities,
		Relations: relations,
		Summary: jsoncontract.QueryScopeSummary{
			Total:  len(r.Entities),
			ByType: byType,
		},
	}
}

func init() {
	queryCmd.AddCommand(queryScopeCmd)
	queryCmd.AddCommand(queryPathCmd)
	queryUnresolvedCmd.Flags().String("type", "", "filter by entity type (question, assumption, risk)")
	queryCmd.AddCommand(queryUnresolvedCmd)
}

var validUnresolvedTypes = map[string]model.EntityType{
	"question":   model.EntityTypeQuestion,
	"assumption": model.EntityTypeAssumption,
	"risk":       model.EntityTypeRisk,
}

var queryUnresolvedCmd = &cobra.Command{
	Use:   "unresolved",
	Short: "List unresolved questions, assumptions, and risks",
	RunE: func(cmd *cobra.Command, args []string) error {
		typeFlag, _ := cmd.Flags().GetString("type")

		var opts graph.QueryUnresolvedOptions
		if typeFlag != "" {
			et, ok := validUnresolvedTypes[typeFlag]
			if !ok {
				handleError(cmd, &model.ErrInvalidInput{
					Message: fmt.Sprintf("invalid --type %q; must be question, assumption, or risk", typeFlag),
				})
				return nil
			}
			opts.Type = &et
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)

		result, err := graph.QueryUnresolved(opts, &entityStoreAdapter{store: es})
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		response := convertQueryUnresolvedResult(result)
		writeJSON(cmd, response)
		return nil
	},
}

func convertQueryUnresolvedResult(r *graph.QueryUnresolvedResult) jsoncontract.QueryUnresolvedResponse {
	entities := make([]jsoncontract.EntitySummary, 0, len(r.Entities))
	for _, e := range r.Entities {
		entities = append(entities, jsoncontract.EntitySummary{
			ID:     e.ID,
			Type:   string(e.Type),
			Title:  e.Title,
			Status: string(e.Status),
		})
	}

	byType := make(map[string]int)
	for _, e := range r.Entities {
		byType[string(e.Type)]++
	}

	return jsoncontract.QueryUnresolvedResponse{
		Entities: entities,
		Summary: jsoncontract.QueryUnresolvedSummary{
			Total:  len(r.Entities),
			ByType: byType,
		},
	}
}

var queryPathCmd = &cobra.Command{
	Use:   "path <from-id> <to-id>",
	Short: "Find shortest path between two entities",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		toID := args[1]

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		rs := store.NewRelationStore(db, cs, hs)
		es := store.NewEntityStore(db, cs, hs)

		opts := graph.QueryPathOptions{FromID: fromID, ToID: toID}
		result, err := graph.QueryPath(opts, rs, &entityStoreAdapter{store: es})
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		response := convertQueryPathResult(result)
		writeJSON(cmd, response)
		return nil
	},
}

func convertQueryPathResult(r *graph.QueryPathResult) jsoncontract.QueryPathResponse {
	steps := make([]jsoncontract.PathStep, 0, len(r.Path))
	for _, n := range r.Path {
		steps = append(steps, jsoncontract.PathStep{
			EntityID:   n.EntityID,
			EntityType: string(n.EntityType),
			Relation:   string(n.Relation),
		})
	}

	length := 0
	if len(r.Path) > 1 {
		length = len(r.Path) - 1
	}

	return jsoncontract.QueryPathResponse{
		From:   r.FromID,
		To:     r.ToID,
		Found:  r.Found,
		Path:   steps,
		Length: length,
	}
}

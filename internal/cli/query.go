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

		layerStr, err := ParseLayerFlagString(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		result, err := engine.QueryScope(cmd.Context(), specgraph.QueryScopeRequest{
			PhaseID: phaseID,
			Layer:   layerStr,
		})
		if err != nil {
			return handleError(cmd, err)
		}

		response := convertQueryScopeResult(result)
		return writeJSON(cmd, response)
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
	queryUnresolvedCmd.Flags().String("phase", "", "restrict results to entities in the scope of this phase")
	queryCmd.AddCommand(queryUnresolvedCmd)
	queryCmd.AddCommand(querySQLCmd)
	queryNeighborsCmd.Flags().Int("depth", 1, "traversal depth (0 = center only)")
	queryCmd.AddCommand(queryNeighborsCmd)
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
		phaseFlag, _ := cmd.Flags().GetString("phase")

		if typeFlag != "" {
			if _, ok := validUnresolvedTypes[typeFlag]; !ok {
				return handleError(cmd, &model.ErrInvalidInput{
					Message: fmt.Sprintf("invalid --type %q; must be question, assumption, or risk", typeFlag),
				})
			}
		}

		result, err := engine.QueryUnresolved(cmd.Context(), specgraph.QueryUnresolvedRequest{Type: typeFlag})
		if err != nil {
			return handleError(cmd, err)
		}

		if phaseFlag != "" {
			phaseScope, scopeErr := phaseEntityScope(phaseFlag, engine.RelationsByEntity)
			if scopeErr != nil {
				return handleError(cmd, &model.ErrInvalidInput{
					Message: fmt.Sprintf("invalid --phase %q: %v", phaseFlag, scopeErr),
				})
			}
			filtered := make([]model.Entity, 0, len(result.Entities))
			for _, e := range result.Entities {
				if phaseScope[e.ID] {
					filtered = append(filtered, e)
				}
			}
			result.Entities = filtered
			result.Count = len(filtered)
		}

		response := convertQueryUnresolvedResult(result)
		return writeJSON(cmd, response)
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

		layerStr, err := ParseLayerFlagString(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		result, err := engine.QueryPath(cmd.Context(), specgraph.QueryPathRequest{
			FromID: fromID,
			ToID:   toID,
			Layer:  layerStr,
		})
		if err != nil {
			return handleError(cmd, err)
		}

		response := convertQueryPathResult(result)
		return writeJSON(cmd, response)
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

var queryNeighborsCmd = &cobra.Command{
	Use:   "neighbors <entity-id>",
	Short: "Find neighboring entities within a given depth",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		depth, _ := cmd.Flags().GetInt("depth")

		result, err := engine.QueryNeighbors(cmd.Context(), specgraph.QueryNeighborsRequest{
			EntityID: entityID,
			Depth:    depth,
		})
		if err != nil {
			return handleError(cmd, err)
		}

		response := convertNeighborsResult(result)
		return writeJSON(cmd, response)
	},
}

func convertNeighborsResult(r *graph.NeighborResult) jsoncontract.QueryNeighborsResponse {
	entities := make([]jsoncontract.NeighborEntityResponse, 0, len(r.Entities))
	for _, ne := range r.Entities {
		entities = append(entities, jsoncontract.NeighborEntityResponse{
			ID:     ne.Entity.ID,
			Type:   string(ne.Entity.Type),
			Title:  ne.Entity.Title,
			Status: string(ne.Entity.Status),
			Depth:  ne.Depth,
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

	return jsoncontract.QueryNeighborsResponse{
		Center:    r.Center,
		Entities:  entities,
		Relations: relations,
	}
}

var querySQLCmd = &cobra.Command{
	Use:   "sql <query>",
	Short: "Execute a read-only SQL query against the graph database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.TrimSpace(args[0])
		upper := strings.ToUpper(query)

		if !strings.HasPrefix(upper, "SELECT") {
			return handleError(cmd, &model.ErrInvalidInput{Message: "only SELECT statements are allowed"})
		}

		res, err := engine.RawQuery(cmd.Context(), query)
		if err != nil {
			return handleError(cmd, err)
		}

		return writeJSON(cmd, jsoncontract.QuerySQLResponse{
			Columns: res.Columns,
			Rows:    res.Rows,
			Count:   len(res.Rows),
		})
	},
}

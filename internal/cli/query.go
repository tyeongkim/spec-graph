package cli

import (
	"context"
	"fmt"
	"strings"

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

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		opts := graph.QueryScopeOptions{PhaseID: phaseID, Layer: layer}
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

		layer, err := ParseLayerFlag(cmd)
		if err != nil {
			handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
			return nil
		}

		opts := graph.QueryPathOptions{FromID: fromID, ToID: toID, Layer: layer}
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

var queryNeighborsCmd = &cobra.Command{
	Use:   "neighbors <entity-id>",
	Short: "Find neighboring entities within a given depth",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		depth, _ := cmd.Flags().GetInt("depth")

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		rs := store.NewRelationStore(db, cs, hs)
		es := store.NewEntityStore(db, cs, hs)

		result, err := graph.Neighbors(entityID, depth, rs, &entityStoreAdapter{store: es})
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		response := convertNeighborsResult(result)
		writeJSON(cmd, response)
		return nil
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

var forbiddenSQLPrefixes = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "PRAGMA",
}

var querySQLCmd = &cobra.Command{
	Use:   "sql <query>",
	Short: "Execute a read-only SQL query against the graph database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.TrimSpace(args[0])
		upper := strings.ToUpper(query)

		if !strings.HasPrefix(upper, "SELECT") {
			handleError(cmd, &model.ErrInvalidInput{Message: "only SELECT statements are allowed"})
			return nil
		}
		for _, prefix := range forbiddenSQLPrefixes {
			if strings.HasPrefix(upper, prefix) {
				handleError(cmd, &model.ErrInvalidInput{Message: "only SELECT statements are allowed"})
				return nil
			}
		}

		rows, err := getDB().QueryContext(context.Background(), query)
		if err != nil {
			handleError(cmd, fmt.Errorf("query execution: %w", err))
			return nil
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			handleError(cmd, fmt.Errorf("read columns: %w", err))
			return nil
		}

		var result []map[string]interface{}
		for rows.Next() {
			vals := make([]interface{}, len(columns))
			ptrs := make([]interface{}, len(columns))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				handleError(cmd, fmt.Errorf("scan row: %w", err))
				return nil
			}
			row := make(map[string]interface{}, len(columns))
			for i, col := range columns {
				switch v := vals[i].(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = v
				}
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			handleError(cmd, fmt.Errorf("iterate rows: %w", err))
			return nil
		}

		if result == nil {
			result = []map[string]interface{}{}
		}

		writeJSON(cmd, jsoncontract.QuerySQLResponse{
			Columns: columns,
			Rows:    result,
			Count:   len(result),
		})
		return nil
	},
}

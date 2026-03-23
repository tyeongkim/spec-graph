package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/graph"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the spec graph in DOT, Mermaid, or JSON format",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		centerFlag, _ := cmd.Flags().GetString("center")
		depthFlag, _ := cmd.Flags().GetInt("depth")

		switch format {
		case "dot", "mermaid", "json":
			// valid
		default:
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("unknown format %q; must be dot, mermaid, or json", format),
			})
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		rs := store.NewRelationStore(db, cs, hs)

		var entities []model.Entity
		var relations []model.Relation

		if centerFlag != "" {
			result, err := graph.Neighbors(centerFlag, depthFlag, rs, &entityStoreAdapter{store: es})
			if err != nil {
				handleError(cmd, err)
				return nil
			}
			entities = make([]model.Entity, len(result.Entities))
			for i, ne := range result.Entities {
				entities[i] = ne.Entity
			}
			relations = result.Relations
		} else {
			var err error
			entities, _, err = es.List(store.EntityFilters{})
			if err != nil {
				handleError(cmd, err)
			}
			relations, _, err = rs.List(store.RelationFilters{})
			if err != nil {
				handleError(cmd, err)
			}
		}

		if format == "json" {
			result := graph.ExportJSON(entities, relations)
			writeJSON(cmd, result)
		} else {
			var output string
			if format == "dot" {
				output = graph.ExportDOT(entities, relations)
			} else {
				output = graph.ExportMermaid(entities, relations)
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
		}
		return nil
	},
}

func init() {
	exportCmd.Flags().String("format", "", "export format: dot, mermaid, or json (required)")
	_ = exportCmd.MarkFlagRequired("format")
	exportCmd.Flags().String("center", "", "center entity ID for subgraph export")
	exportCmd.Flags().Int("depth", 2, "traversal depth from center entity (default: 2)")
}

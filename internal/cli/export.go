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
	Short: "Export the spec graph in DOT or Mermaid format",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")

		switch format {
		case "dot", "mermaid":
			// valid
		default:
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("unknown format %q; must be dot or mermaid", format),
			})
		}

		db := getDB()
		cs := store.NewChangesetStore(db)
		hs := store.NewHistoryStore(db)
		es := store.NewEntityStore(db, cs, hs)
		rs := store.NewRelationStore(db, cs, hs)

		entities, _, err := es.List(store.EntityFilters{})
		if err != nil {
			handleError(cmd, err)
		}

		relations, _, err := rs.List(store.RelationFilters{})
		if err != nil {
			handleError(cmd, err)
		}

		var output string
		if format == "dot" {
			output = graph.ExportDOT(entities, relations)
		} else {
			output = graph.ExportMermaid(entities, relations)
		}

		fmt.Fprint(cmd.OutOrStdout(), output)
		return nil
	},
}

func init() {
	exportCmd.Flags().String("format", "", "export format: dot or mermaid (required)")
	_ = exportCmd.MarkFlagRequired("format")
}

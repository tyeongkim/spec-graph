package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the spec graph in DOT, Mermaid, or JSON format",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		centerFlag, _ := cmd.Flags().GetString("center")
		depthFlag, _ := cmd.Flags().GetInt("depth")

		layerStr, err := ParseLayerFlagString(cmd)
		if err != nil {
			return handleError(cmd, &model.ErrInvalidInput{Message: err.Error()})
		}

		switch format {
		case "dot", "mermaid", "json":
		default:
			return handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("unknown format %q; must be dot, mermaid, or json", format),
			})
		}

		result, err := engine.Export(cmd.Context(), specgraph.ExportRequest{
			Format: format,
			Center: centerFlag,
			Depth:  depthFlag,
			Layer:  layerStr,
		})
		if err != nil {
			return handleError(cmd, err)
		}

		if format == "json" {
			var jsonResult jsoncontract.ExportJSONResult
			if err := json.Unmarshal([]byte(result.Data), &jsonResult); err != nil {
				return handleError(cmd, fmt.Errorf("decode export json: %w", err))
			}
			return writeJSON(cmd, jsonResult)
		}
		fmt.Fprint(cmd.OutOrStdout(), result.Data)
		return nil
	},
}

func init() {
	exportCmd.Flags().String("format", "", "export format: dot, mermaid, or json (required)")
	_ = exportCmd.MarkFlagRequired("format")
	exportCmd.Flags().String("center", "", "center entity ID for subgraph export")
	exportCmd.Flags().Int("depth", 2, "traversal depth from center entity (default: 2)")
}

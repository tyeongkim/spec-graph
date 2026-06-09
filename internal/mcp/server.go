package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func NewSpecGraphServer(engine *specgraph.Engine) *server.MCPServer {
	s := server.NewMCPServer("spec-graph", "0.5.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(queryScope(), handleQueryScope(engine))
	s.AddTool(queryPath(), handleQueryPath(engine))
	s.AddTool(queryUnresolved(), handleQueryUnresolved(engine))
	s.AddTool(impactTool(), handleImpact(engine))
	s.AddTool(validateTool(), handleValidate(engine))
	s.AddTool(exportTool(), handleExport(engine))

	return s
}

// --- Tool definitions ---

func queryScope() mcp.Tool {
	return mcp.NewTool("query_scope",
		mcp.WithDescription("List entities and relations belonging to a phase"),
		mcp.WithString("phase_id",
			mcp.Required(),
			mcp.Description("The phase entity ID to scope by"),
		),
		mcp.WithString("layer",
			mcp.Description("Filter by layer: arch, exec, mapping, or all (default)"),
		),
	)
}

func queryPath() mcp.Tool {
	return mcp.NewTool("query_path",
		mcp.WithDescription("Find shortest path between two entities"),
		mcp.WithString("from_id",
			mcp.Required(),
			mcp.Description("Source entity ID"),
		),
		mcp.WithString("to_id",
			mcp.Required(),
			mcp.Description("Target entity ID"),
		),
		mcp.WithString("layer",
			mcp.Description("Filter by layer: arch, exec, mapping, or all (default)"),
		),
	)
}

func queryUnresolved() mcp.Tool {
	return mcp.NewTool("query_unresolved",
		mcp.WithDescription("List unresolved questions, assumptions, and risks"),
		mcp.WithString("type",
			mcp.Description("Filter by entity type: question, assumption, or risk"),
		),
	)
}

func impactTool() mcp.Tool {
	return mcp.NewTool("impact",
		mcp.WithDescription("Analyze impact of changes from source entities"),
		mcp.WithString("sources",
			mcp.Required(),
			mcp.Description("Comma-separated source entity IDs"),
		),
		mcp.WithString("dimension",
			mcp.Description("Restrict scoring to single dimension: structural, behavioral, or planning"),
		),
		mcp.WithString("min_severity",
			mcp.Description("Minimum severity filter: high, medium, or low"),
		),
		mcp.WithString("layer",
			mcp.Description("Filter by layer: arch, exec, mapping, or all (default)"),
		),
	)
}

func validateTool() mcp.Tool {
	return mcp.NewTool("validate",
		mcp.WithDescription("Validate the specification graph"),
		mcp.WithString("check",
			mcp.Description("Comma-separated check names: orphans, coverage, invalid_edges, superseded_refs, gates"),
		),
		mcp.WithString("phase",
			mcp.Description("Restrict validation to entities in this phase (must be a phase entity ID)"),
		),
		mcp.WithString("layer",
			mcp.Description("Filter by layer: arch, exec, mapping, or all (default)"),
		),
	)
}

func exportTool() mcp.Tool {
	return mcp.NewTool("export",
		mcp.WithDescription("Export the spec graph in DOT or Mermaid format"),
		mcp.WithString("format",
			mcp.Required(),
			mcp.Description("Export format"),
			mcp.Enum("dot", "mermaid"),
		),
		mcp.WithString("layer",
			mcp.Description("Filter by layer: arch, exec, mapping, or all (default)"),
		),
	)
}

// --- Handlers ---

func handleQueryScope(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		phaseID, err := req.RequireString("phase_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := engine.QueryScope(ctx, specgraph.QueryScopeRequest{
			PhaseID: phaseID,
			Layer:   layerToString(layer),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleQueryPath(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fromID, err := req.RequireString("from_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		toID, err := req.RequireString("to_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := engine.QueryPath(ctx, specgraph.QueryPathRequest{
			FromID: fromID,
			ToID:   toID,
			Layer:  layerToString(layer),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

var validUnresolvedTypes = map[string]model.EntityType{
	"question":   model.EntityTypeQuestion,
	"assumption": model.EntityTypeAssumption,
	"risk":       model.EntityTypeRisk,
}

func handleQueryUnresolved(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typeStr := req.GetString("type", "")

		if typeStr != "" {
			if _, ok := validUnresolvedTypes[typeStr]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; must be question, assumption, or risk", typeStr)), nil
			}
		}

		result, err := engine.QueryUnresolved(ctx, specgraph.QueryUnresolvedRequest{Type: typeStr})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleImpact(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourcesStr, err := req.RequireString("sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources := splitCSV(sourcesStr)
		if len(sources) == 0 {
			return mcp.NewToolResultError("sources must not be empty"), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dimStr := req.GetString("dimension", "")
		if dimStr != "" {
			switch dimStr {
			case "structural", "behavioral", "planning":
			default:
				return mcp.NewToolResultError(fmt.Sprintf("invalid dimension %q; must be structural, behavioral, or planning", dimStr)), nil
			}
		}

		minSevStr := req.GetString("min_severity", "")
		if minSevStr != "" {
			switch minSevStr {
			case "high", "medium", "low":
			default:
				return mcp.NewToolResultError(fmt.Sprintf("invalid min_severity %q; must be high, medium, or low", minSevStr)), nil
			}
		}

		result, err := engine.Impact(ctx, specgraph.ImpactRequest{
			Sources:     sources,
			Dimension:   dimStr,
			MinSeverity: minSevStr,
			Layer:       layerToString(layer),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleValidate(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		checkStr := req.GetString("check", "")
		phaseStr := req.GetString("phase", "")

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if phaseStr != "" && layer != nil && *layer != model.LayerMapping {
			return mcp.NewToolResultError(fmt.Sprintf("phase cannot be used with layer %s; only layer mapping or all is allowed", *layer)), nil
		}

		var checks []string
		if checkStr != "" {
			checks = splitCSV(checkStr)
		}

		result, err := engine.Validate(ctx, specgraph.ValidateRequest{
			Checks: checks,
			Phase:  phaseStr,
			Layer:  layerToString(layer),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleExport(engine *specgraph.Engine) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		format, err := req.RequireString("format")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := engine.Export(ctx, specgraph.ExportRequest{
			Format: format,
			Layer:  layerToString(layer),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result.Data), nil
	}
}

// --- Helpers ---

func parseLayerParam(req mcp.CallToolRequest) (*model.Layer, error) {
	val := req.GetString("layer", "")
	if val == "" || val == "all" {
		return nil, nil
	}
	l := model.Layer(val)
	if !model.IsValidLayer(l) {
		return nil, fmt.Errorf("invalid layer %q: valid values are arch, exec, mapping, all", val)
	}
	return &l, nil
}

// layerToString converts an optional layer pointer into the string form the
// engine expects: an empty string means all layers.
func layerToString(layer *model.Layer) string {
	if layer == nil {
		return ""
	}
	return string(*layer)
}

func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %s", err.Error())), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

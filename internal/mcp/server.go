// Package mcp provides an MCP (Model Context Protocol) server exposing
// spec-graph read-only query, validation, impact, and export tools.
package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/store"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// entityStoreAdapter wraps *store.EntityStore to implement graph.EntityFetcher.
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

type validateEntityAdapter struct {
	store *store.EntityStore
}

func (a *validateEntityAdapter) Get(id string) (model.Entity, error) {
	return a.store.Get(id)
}

func (a *validateEntityAdapter) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	sf := store.EntityFilters{Type: filters.Type, Status: filters.Status, Layer: filters.Layer}
	entities, _, err := a.store.List(sf)
	return entities, err
}

// newStores creates the store instances needed by graph functions.
func newStores(db *sql.DB) (*store.RelationStore, *entityStoreAdapter) {
	cs := store.NewChangesetStore(db)
	hs := store.NewHistoryStore(db)
	rs := store.NewRelationStore(db, cs, hs)
	es := store.NewEntityStore(db, cs, hs)
	return rs, &entityStoreAdapter{store: es}
}

// NewSpecGraphServer creates a configured MCP server with 6 read-only tools.
func NewSpecGraphServer(db *sql.DB) *server.MCPServer {
	s := server.NewMCPServer("spec-graph", "0.5.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(queryScope(), handleQueryScope(db))
	s.AddTool(queryPath(), handleQueryPath(db))
	s.AddTool(queryUnresolved(), handleQueryUnresolved(db))
	s.AddTool(impactTool(), handleImpact(db))
	s.AddTool(validateTool(), handleValidate(db))
	s.AddTool(exportTool(), handleExport(db))

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

func handleQueryScope(db *sql.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		phaseID, err := req.RequireString("phase_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		rs, ef := newStores(db)
		result, err := graph.QueryScope(graph.QueryScopeOptions{PhaseID: phaseID, Layer: layer}, rs, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleQueryPath(db *sql.DB) server.ToolHandlerFunc {
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

		rs, ef := newStores(db)
		result, err := graph.QueryPath(graph.QueryPathOptions{FromID: fromID, ToID: toID, Layer: layer}, rs, ef)
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

func handleQueryUnresolved(db *sql.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typeStr := req.GetString("type", "")

		var opts graph.QueryUnresolvedOptions
		if typeStr != "" {
			et, ok := validUnresolvedTypes[typeStr]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; must be question, assumption, or risk", typeStr)), nil
			}
			opts.Type = &et
		}

		_, ef := newStores(db)
		result, err := graph.QueryUnresolved(opts, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleImpact(db *sql.DB) server.ToolHandlerFunc {
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

		var opts graph.ImpactOptions
		opts.Layer = layer

		dimStr := req.GetString("dimension", "")
		if dimStr != "" {
			switch dimStr {
			case "structural", "behavioral", "planning":
				opts.Dimension = &dimStr
			default:
				return mcp.NewToolResultError(fmt.Sprintf("invalid dimension %q; must be structural, behavioral, or planning", dimStr)), nil
			}
		}

		minSevStr := req.GetString("min_severity", "")
		if minSevStr != "" {
			switch minSevStr {
			case "high", "medium", "low":
				sev := graph.Severity(minSevStr)
				opts.MinSeverity = &sev
			default:
				return mcp.NewToolResultError(fmt.Sprintf("invalid min_severity %q; must be high, medium, or low", minSevStr)), nil
			}
		}

		rs, ef := newStores(db)
		result, err := graph.Impact(sources, opts, rs, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleValidate(db *sql.DB) server.ToolHandlerFunc {
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

		var opts validate.ValidateOptions
		opts.Layer = layer
		if checkStr != "" {
			opts.Checks = splitCSV(checkStr)
		}
		if phaseStr != "" {
			opts.Phase = &phaseStr
		}

		rs, ef := newStores(db)
		vef := &validateEntityAdapter{store: ef.store}
		result, err := validate.Validate(opts, rs, vef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleExport(db *sql.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		format, err := req.RequireString("format")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		rs, ef := newStores(db)
		opts := &graph.ExportOptions{Layer: layer}

		entities, listErr := ef.List(graph.EntityListFilters{})
		if listErr != nil {
			return mcp.NewToolResultError(listErr.Error()), nil
		}

		allRels, relErr := collectAllRelations(rs, entities)
		if relErr != nil {
			return mcp.NewToolResultError(relErr.Error()), nil
		}

		var output string
		switch format {
		case "dot":
			output = graph.ExportDOT(entities, allRels, opts)
		case "mermaid":
			output = graph.ExportMermaid(entities, allRels, opts)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unknown format %q; must be dot or mermaid", format)), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

// --- Helpers ---

// collectAllRelations gathers deduplicated relations for all entities.
func collectAllRelations(rs *store.RelationStore, entities []model.Entity) ([]model.Relation, error) {
	seen := make(map[int]bool)
	var all []model.Relation
	for _, e := range entities {
		rels, err := rs.GetByEntity(e.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range rels {
			if !seen[r.ID] {
				seen[r.ID] = true
				all = append(all, r)
			}
		}
	}
	return all, nil
}

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

// marshalResult serializes v to JSON and returns it as a tool text result.
func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %s", err.Error())), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
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

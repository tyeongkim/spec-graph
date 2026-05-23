package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type entityFetcherAdapter struct {
	idx *index.Index
}

func (a *entityFetcherAdapter) Get(id string) (model.Entity, error) {
	rec, err := a.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return entityFromRecord(rec), nil
}

func (a *entityFetcherAdapter) List(filters graph.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	recs, err := a.idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	return entitiesToModel(recs), nil
}

type validateEntityFetcherAdapter struct {
	idx *index.Index
}

func (a *validateEntityFetcherAdapter) Get(id string) (model.Entity, error) {
	rec, err := a.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return entityFromRecord(rec), nil
}

func (a *validateEntityFetcherAdapter) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	if filters.Layer != nil {
		ef.Layer = string(*filters.Layer)
	}
	recs, err := a.idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	return entitiesToModel(recs), nil
}

type relationFetcherAdapter struct {
	idx *index.Index
}

func (a *relationFetcherAdapter) GetByEntity(entityID string) ([]model.Relation, error) {
	recs, err := a.idx.GetRelationsByEntity(entityID)
	if err != nil {
		return nil, err
	}
	return relationsToModel(recs), nil
}

func entityFromRecord(rec *index.EntityRecord) model.Entity {
	return model.Entity{
		ID:          rec.ID,
		Type:        model.EntityType(rec.Type),
		Layer:       model.Layer(rec.Layer),
		Status:      model.EntityStatus(rec.Status),
		Title:       rec.Title,
		Description: rec.Description,
		Metadata:    json.RawMessage(rec.Metadata),
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

func entitiesToModel(recs []index.EntityRecord) []model.Entity {
	entities := make([]model.Entity, len(recs))
	for i := range recs {
		entities[i] = entityFromRecord(&recs[i])
	}
	return entities
}

func relationsToModel(recs []index.RelationRecord) []model.Relation {
	rels := make([]model.Relation, len(recs))
	for i := range recs {
		rels[i] = model.Relation{
			FromID:   recs[i].FromID,
			ToID:     recs[i].ToID,
			Type:     model.RelationType(recs[i].Type),
			Layer:    model.Layer(recs[i].Layer),
			Weight:   recs[i].Weight,
			Metadata: json.RawMessage(recs[i].Metadata),
		}
	}
	return rels
}

func NewSpecGraphServer(idx *index.Index) *server.MCPServer {
	s := server.NewMCPServer("spec-graph", "0.5.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(queryScope(), handleQueryScope(idx))
	s.AddTool(queryPath(), handleQueryPath(idx))
	s.AddTool(queryUnresolved(), handleQueryUnresolved(idx))
	s.AddTool(impactTool(), handleImpact(idx))
	s.AddTool(validateTool(), handleValidate(idx))
	s.AddTool(exportTool(), handleExport(idx))

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

func handleQueryScope(idx *index.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		phaseID, err := req.RequireString("phase_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		ef := &entityFetcherAdapter{idx: idx}
		rf := &relationFetcherAdapter{idx: idx}
		result, err := graph.QueryScope(graph.QueryScopeOptions{PhaseID: phaseID, Layer: layer}, rf, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleQueryPath(idx *index.Index) server.ToolHandlerFunc {
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

		ef := &entityFetcherAdapter{idx: idx}
		rf := &relationFetcherAdapter{idx: idx}
		result, err := graph.QueryPath(graph.QueryPathOptions{FromID: fromID, ToID: toID, Layer: layer}, rf, ef)
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

func handleQueryUnresolved(idx *index.Index) server.ToolHandlerFunc {
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

		ef := &entityFetcherAdapter{idx: idx}
		result, err := graph.QueryUnresolved(opts, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleImpact(idx *index.Index) server.ToolHandlerFunc {
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

		ef := &entityFetcherAdapter{idx: idx}
		rf := &relationFetcherAdapter{idx: idx}
		result, err := graph.Impact(sources, opts, rf, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleValidate(idx *index.Index) server.ToolHandlerFunc {
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

		ef := &validateEntityFetcherAdapter{idx: idx}
		rf := &relationFetcherAdapter{idx: idx}
		result, err := validate.Validate(opts, rf, ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(result)
	}
}

func handleExport(idx *index.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		format, err := req.RequireString("format")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		layer, err := parseLayerParam(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		opts := &graph.ExportOptions{Layer: layer}

		var ef index.EntityFilters
		if layer != nil {
			ef.Layer = string(*layer)
		}
		entityRecs, err := idx.ListEntities(ef)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		entities := entitiesToModel(entityRecs)

		var rf index.RelationFilters
		if layer != nil {
			rf.Layer = string(*layer)
		}
		relRecs, err := idx.ListRelations(rf)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		relations := relationsToModel(relRecs)

		var output string
		switch format {
		case "dot":
			output = graph.ExportDOT(entities, relations, opts)
		case "mermaid":
			output = graph.ExportMermaid(entities, relations, opts)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unknown format %q; must be dot or mermaid", format)), nil
		}

		return mcp.NewToolResultText(output), nil
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

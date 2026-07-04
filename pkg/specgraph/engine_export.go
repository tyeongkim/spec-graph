package specgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// ExportRequest describes the inputs for exporting the graph.
type ExportRequest struct {
	// Format selects the output format. Required. One of "json", "dot", "mermaid".
	Format string
	// Center, when non-empty, exports only the subgraph around this entity ID.
	Center string
	// Depth bounds the traversal distance from Center. Defaults to 3 when Center
	// is set and Depth is 0.
	Depth int
	// Layer restricts output to entities in this layer. Empty = all layers.
	// One of "arch", "exec", "mapping".
	Layer string
}

// ExportResult holds the rendered output of an export.
type ExportResult struct {
	// Format echoes the format that was used.
	Format string
	// Data holds the rendered output: DOT or Mermaid text, or a JSON string for
	// the "json" format.
	Data string
}

// Export renders the graph in the requested format. It validates the format,
// optionally restricts output to the subgraph around req.Center (bounded by
// req.Depth) or to a single layer, gathers the matching entities and
// relations, and delegates to the corresponding graph export function. The
// provided context is accepted for forward compatibility and is not yet
// observed.
func (e *Engine) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	_ = ctx

	return readLocked(e, func() (*ExportResult, error) {
		return e.exportLocked(req)
	})
}

func (e *Engine) exportLocked(req ExportRequest) (*ExportResult, error) {
	switch req.Format {
	case "json", "dot", "mermaid":
	default:
		return nil, newError(CodeInvalidInput, fmt.Sprintf("unknown format %q; must be json, dot, or mermaid", req.Format), nil)
	}

	var layer *model.Layer
	if req.Layer != "" {
		l := model.Layer(req.Layer)
		layer = &l
	}

	opts := &graph.ExportOptions{Layer: layer}

	var entities []model.Entity
	var relations []model.Relation

	if req.Center != "" {
		depth := req.Depth
		if depth == 0 {
			depth = 3
		}

		ef := &engineGraphEntityFetcher{idx: e.idx}
		rf := &engineRelationFetcher{idx: e.idx}

		result, err := graph.Neighbors(req.Center, depth, rf, ef)
		if err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("neighbors of %q", req.Center), err)
		}

		entities = make([]model.Entity, len(result.Entities))
		for i, ne := range result.Entities {
			entities[i] = ne.Entity
		}
		relations = result.Relations
	} else {
		var entFilters index.EntityFilters
		if layer != nil {
			entFilters.Layer = string(*layer)
		}
		entRecs, err := e.idx.ListEntities(entFilters)
		if err != nil {
			return nil, newError(CodeRuntime, "list entities", err)
		}
		entities = make([]model.Entity, len(entRecs))
		for i := range entRecs {
			entities[i] = engineEntityFromRecord(&entRecs[i])
		}

		var relFilters index.RelationFilters
		if layer != nil {
			relFilters.Layer = string(*layer)
		}
		relRecs, err := e.idx.ListRelations(relFilters)
		if err != nil {
			return nil, newError(CodeRuntime, "list relations", err)
		}
		relations = make([]model.Relation, len(relRecs))
		for i := range relRecs {
			rec := &relRecs[i]
			relations[i] = model.Relation{
				FromID:   rec.FromID,
				ToID:     rec.ToID,
				Type:     model.RelationType(rec.Type),
				Layer:    model.Layer(rec.Layer),
				Weight:   rec.Weight,
				Metadata: json.RawMessage(rec.Metadata),
			}
		}
	}

	var output string
	switch req.Format {
	case "dot":
		output = graph.ExportDOT(entities, relations, opts)
	case "mermaid":
		output = graph.ExportMermaid(entities, relations, opts)
	case "json":
		jsonResult := graph.ExportJSON(entities, relations, opts)
		data, err := json.Marshal(jsonResult)
		if err != nil {
			return nil, newError(CodeRuntime, "marshal export json", err)
		}
		output = string(data)
	}

	return &ExportResult{Format: req.Format, Data: output}, nil
}

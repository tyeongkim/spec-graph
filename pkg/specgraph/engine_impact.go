package specgraph

import (
	"context"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// ImpactRequest describes the inputs for an impact analysis traversal.
type ImpactRequest struct {
	// Sources lists the source entity IDs to start traversal from. Required,
	// at least one element.
	Sources []string
	// Follow restricts traversal to these relation types. nil/empty = all types.
	Follow []string
	// MinSeverity filters out affected entities below this severity. Empty = no
	// filter. One of "high", "medium", "low".
	MinSeverity string
	// Dimension restricts scoring to a single dimension. Empty = all dimensions.
	// One of "structural", "behavioral", "planning".
	Dimension string
	// Layer restricts traversal to entities in this layer. Empty = all layers.
	// One of "arch", "exec", "mapping".
	Layer string
}

// Impact performs priority-queue BFS impact analysis from the source entities
// in req, returning all transitively affected entities with per-dimension
// scores, severity, and path information. It validates that at least one
// source is provided, converts the request filters into graph options, and
// delegates to graph.Impact using index-backed fetchers. The provided context
// is accepted for forward compatibility and is not yet observed.
func (e *Engine) Impact(ctx context.Context, req ImpactRequest) (*graph.ImpactResult, error) {
	_ = ctx

	if len(req.Sources) == 0 {
		return nil, newError(CodeInvalidInput, "at least one source is required", nil)
	}

	var follow []model.RelationType
	if len(req.Follow) > 0 {
		follow = make([]model.RelationType, len(req.Follow))
		for i, f := range req.Follow {
			follow[i] = model.RelationType(f)
		}
	}

	var minSeverity *graph.Severity
	if req.MinSeverity != "" {
		sev := graph.Severity(req.MinSeverity)
		minSeverity = &sev
	}

	var dimension *string
	if req.Dimension != "" {
		dim := req.Dimension
		dimension = &dim
	}

	var layer *model.Layer
	if req.Layer != "" {
		l := model.Layer(req.Layer)
		layer = &l
	}

	opts := graph.ImpactOptions{
		Follow:      follow,
		MinSeverity: minSeverity,
		Dimension:   dimension,
		Layer:       layer,
	}

	ef := &engineGraphEntityFetcher{idx: e.idx}
	rf := &engineRelationFetcher{idx: e.idx}

	result, err := graph.Impact(req.Sources, opts, rf, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "impact analysis", err)
	}

	return result, nil
}

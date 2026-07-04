package specgraph

import (
	"context"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// QueryScopeRequest describes the inputs for scoping a query to a phase.
type QueryScopeRequest struct {
	// PhaseID is the ID of the phase entity to scope by. Required.
	PhaseID string
	// Layer restricts scoped entities to this layer. Empty means all layers.
	Layer string
}

// QueryNeighborsRequest describes the inputs for a neighbor traversal.
type QueryNeighborsRequest struct {
	// EntityID is the center entity to traverse from. Required.
	EntityID string
	// Depth is the maximum BFS depth. Depth 0 returns only the center entity.
	Depth int
}

// QueryPathRequest describes the inputs for a path query between two entities.
type QueryPathRequest struct {
	// FromID is the starting entity ID. Required.
	FromID string
	// ToID is the target entity ID. Required.
	ToID string
	// Layer restricts path traversal to this layer. Empty means all layers.
	Layer string
}

// QueryUnresolvedRequest describes the inputs for an unresolved-entity query.
type QueryUnresolvedRequest struct {
	// Type filters results to a specific entity type. Empty means all
	// unresolved types (question, assumption, risk).
	Type string
}

// engineGraphEntityFetcher adapts the SQLite index to graph.EntityFetcher.
// It differs from engineEntityFetcher in that graph.EntityListFilters has no
// Layer field.
type engineGraphEntityFetcher struct {
	idx *index.Index
}

// Get returns the entity with the given ID, or a not-found error.
func (f *engineGraphEntityFetcher) Get(id string) (model.Entity, error) {
	rec, err := f.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, err
	}
	if rec == nil {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return engineEntityFromRecord(rec), nil
}

// List returns entities matching the given filters.
func (f *engineGraphEntityFetcher) List(filters graph.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	recs, err := f.idx.ListEntities(ef)
	if err != nil {
		return nil, err
	}
	entities := make([]model.Entity, len(recs))
	for i := range recs {
		entities[i] = engineEntityFromRecord(&recs[i])
	}
	return entities, nil
}

// QueryScope returns all entities and relations belonging to the given phase,
// optionally restricted to a single layer. The provided context is accepted for
// forward compatibility and is not yet observed.
func (e *Engine) QueryScope(ctx context.Context, req QueryScopeRequest) (*graph.QueryScopeResult, error) {
	_ = ctx

	return readLocked(e, func() (*graph.QueryScopeResult, error) {
		return e.queryScopeLocked(req)
	})
}

func (e *Engine) queryScopeLocked(req QueryScopeRequest) (*graph.QueryScopeResult, error) {
	if req.PhaseID == "" {
		return nil, newError(CodeInvalidInput, "phase id is required", nil)
	}

	opts := graph.QueryScopeOptions{
		PhaseID: req.PhaseID,
		Layer:   layerPointer(req.Layer),
	}
	ef := &engineGraphEntityFetcher{idx: e.idx}
	rf := &engineRelationFetcher{idx: e.idx}

	result, err := graph.QueryScope(opts, rf, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "query scope", err)
	}
	return result, nil
}

// QueryNeighbors performs a BFS from the requested entity up to the given depth,
// traversing relations in both directions. The provided context is accepted for
// forward compatibility and is not yet observed.
func (e *Engine) QueryNeighbors(ctx context.Context, req QueryNeighborsRequest) (*graph.NeighborResult, error) {
	_ = ctx

	return readLocked(e, func() (*graph.NeighborResult, error) {
		return e.queryNeighborsLocked(req)
	})
}

func (e *Engine) queryNeighborsLocked(req QueryNeighborsRequest) (*graph.NeighborResult, error) {
	if req.EntityID == "" {
		return nil, newError(CodeInvalidInput, "entity id is required", nil)
	}

	ef := &engineGraphEntityFetcher{idx: e.idx}
	rf := &engineRelationFetcher{idx: e.idx}

	result, err := graph.Neighbors(req.EntityID, req.Depth, rf, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "query neighbors", err)
	}
	return result, nil
}

// QueryPath finds the shortest path between two entities, optionally restricted
// to a single layer. The provided context is accepted for forward compatibility
// and is not yet observed.
func (e *Engine) QueryPath(ctx context.Context, req QueryPathRequest) (*graph.QueryPathResult, error) {
	_ = ctx

	return readLocked(e, func() (*graph.QueryPathResult, error) {
		return e.queryPathLocked(req)
	})
}

func (e *Engine) queryPathLocked(req QueryPathRequest) (*graph.QueryPathResult, error) {
	if req.FromID == "" || req.ToID == "" {
		return nil, newError(CodeInvalidInput, "from and to ids are required", nil)
	}

	opts := graph.QueryPathOptions{
		FromID: req.FromID,
		ToID:   req.ToID,
		Layer:  layerPointer(req.Layer),
	}
	ef := &engineGraphEntityFetcher{idx: e.idx}
	rf := &engineRelationFetcher{idx: e.idx}

	result, err := graph.QueryPath(opts, rf, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "query path", err)
	}
	return result, nil
}

// QueryUnresolved returns entities in an unresolved (draft or active) state,
// optionally restricted to a single entity type. The provided context is
// accepted for forward compatibility and is not yet observed.
func (e *Engine) QueryUnresolved(ctx context.Context, req QueryUnresolvedRequest) (*graph.QueryUnresolvedResult, error) {
	_ = ctx

	return readLocked(e, func() (*graph.QueryUnresolvedResult, error) {
		return e.queryUnresolvedLocked(req)
	})
}

func (e *Engine) queryUnresolvedLocked(req QueryUnresolvedRequest) (*graph.QueryUnresolvedResult, error) {
	var typ *model.EntityType
	if req.Type != "" {
		t := model.EntityType(req.Type)
		typ = &t
	}

	opts := graph.QueryUnresolvedOptions{Type: typ}
	ef := &engineGraphEntityFetcher{idx: e.idx}

	result, err := graph.QueryUnresolved(opts, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "query unresolved", err)
	}
	return result, nil
}

// layerPointer converts a layer string into a *model.Layer. An empty string
// yields nil, meaning all layers.
func layerPointer(layer string) *model.Layer {
	if layer == "" {
		return nil
	}
	l := model.Layer(layer)
	return &l
}

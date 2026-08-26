package specgraph

import (
	"context"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// Snapshot reads a graph that is being held still. Every method observes the
// same revision, because the enclosing Read call holds both the cross-process
// file lock and the in-process read lock for its whole duration.
//
// A Snapshot is only valid inside the Read call that produced it; retaining one
// past that call reads through a lock no longer held.
type Snapshot struct {
	engine *Engine
}

// Read runs fn against a single revision of the graph. Composing reads that must
// agree with each other belongs here rather than in a caller: each standalone
// read method takes and releases the lock on its own, so a sequence of them can
// observe different revisions when another process writes in between.
func (e *Engine) Read(ctx context.Context, fn func(*Snapshot) error) error {
	_, err := readLocked(ctx, e, func() (struct{}, error) {
		return struct{}{}, fn(&Snapshot{engine: e})
	})
	return err
}

// ListEntities returns the entities matching req, along with their count.
func (s *Snapshot) ListEntities(req ListEntitiesRequest) ([]model.Entity, int, error) {
	entities, err := s.engine.listEntitiesLocked(req)
	if err != nil {
		return nil, 0, err
	}
	return entities, len(entities), nil
}

// PhaseContext returns the execution context for phaseID.
func (s *Snapshot) PhaseContext(phaseID string) (PhaseContextResult, error) {
	return s.engine.phaseContextLocked(phaseID)
}

// Validate runs the checks selected by req.
func (s *Snapshot) Validate(req ValidateRequest) (*validate.ValidateResult, error) {
	return s.engine.validateLocked(req)
}

// Impact analyzes what a change to the sources in req affects.
func (s *Snapshot) Impact(req ImpactRequest) (*graph.ImpactResult, error) {
	return s.engine.impactLocked(req)
}

// QueryNeighbors returns the entities within req.Depth of req.EntityID.
func (s *Snapshot) QueryNeighbors(req QueryNeighborsRequest) (*graph.NeighborResult, error) {
	return s.engine.queryNeighborsLocked(req)
}

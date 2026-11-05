package specgraph

import (
	"context"
	"sync"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// Snapshot reads a graph that is being held still. Every method observes the
// same revision, because the enclosing Read call holds both the cross-process
// file lock and the in-process read lock for its whole duration.
//
// A Snapshot is only valid inside the Read call that produced it; retaining one
// past that call causes its methods to return an invalid-state error.
type Snapshot struct {
	state *snapshotState
}

type snapshotState struct {
	engine *Engine
	mu     sync.RWMutex
	closed bool
}

// Read runs fn against a single revision of the graph. Composing reads that must
// agree with each other belongs here rather than in a caller: each standalone
// read method takes and releases the lock on its own, so a sequence of them can
// observe different revisions when another process writes in between.
func (e *Engine) Read(ctx context.Context, fn func(*Snapshot) error) error {
	_, err := readLocked(ctx, e, func() (struct{}, error) {
		snapshot := &Snapshot{state: &snapshotState{engine: e}}
		defer func() {
			snapshot.state.mu.Lock()
			snapshot.state.closed = true
			snapshot.state.mu.Unlock()
		}()
		return struct{}{}, fn(snapshot)
	})
	return err
}

// ListEntities returns the entities matching req, along with their count.
func (s *Snapshot) ListEntities(req ListEntitiesRequest) ([]model.Entity, int, error) {
	if s.state == nil {
		return nil, 0, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return nil, 0, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	entities, err := s.state.engine.listEntitiesLocked(req)
	if err != nil {
		return nil, 0, err
	}
	return entities, len(entities), nil
}

// PhaseContext returns the execution context for phaseID.
func (s *Snapshot) PhaseContext(phaseID string) (PhaseContextResult, error) {
	if s.state == nil {
		return PhaseContextResult{}, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return PhaseContextResult{}, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	return s.state.engine.phaseContextLocked(phaseID)
}

// Validate runs the checks selected by req.
func (s *Snapshot) Validate(req ValidateRequest) (*validate.ValidateResult, error) {
	if s.state == nil {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	return s.state.engine.validateLocked(req)
}

// Impact analyzes what a change to the sources in req affects.
func (s *Snapshot) Impact(req ImpactRequest) (*graph.ImpactResult, error) {
	if s.state == nil {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	return s.state.engine.impactLocked(req)
}

// QueryNeighbors returns the entities within req.Depth of req.EntityID.
func (s *Snapshot) QueryNeighbors(req QueryNeighborsRequest) (*graph.NeighborResult, error) {
	if s.state == nil {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return nil, newError(CodeInvalidState, "snapshot is closed", nil)
	}
	return s.state.engine.queryNeighborsLocked(req)
}

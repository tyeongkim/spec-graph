package specgraph

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tyeongkim/spec-graph/internal/gate"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func (e *Engine) EvaluatePhaseGate(ctx context.Context, phaseID string) (*gate.Report, error) {
	return readLocked(ctx, e, func() (*gate.Report, error) {
		phase, err := e.getEntityLocked(phaseID)
		if err != nil {
			return nil, err
		}
		if phase.Type != model.EntityTypePhase {
			return nil, newError(CodeInvalidInput, fmt.Sprintf("entity %q is type %q, not phase", phaseID, phase.Type), nil)
		}
		candidate := phase
		candidate.Status = model.EntityStatusResolved
		report, err := gate.Enforce(gate.Target{
			EntityID:   phaseID,
			EntityType: phase.Type,
			FromStatus: phase.Status,
			ToStatus:   candidate.Status,
			Candidate:  candidate,
			RepoRoot:   filepath.Dir(e.root),
		}, &engineRelationFetcher{idx: e.idx}, &engineEntityFetcher{idx: e.idx})
		if err != nil {
			return nil, newError(CodeRuntime, "evaluate phase gate", err)
		}
		return report, nil
	})
}

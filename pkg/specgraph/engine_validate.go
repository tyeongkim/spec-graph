package specgraph

import (
	"context"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// ValidateRequest describes the inputs for running validation checks.
type ValidateRequest struct {
	// Checks lists the check names to run. nil means all checks for the
	// selected layer(s).
	Checks []string
	// Phase restricts validation to entities belonging to this phase. Empty
	// means all entities.
	Phase string
	// EntityID restricts reported issues to this specific entity. Empty means
	// all entities.
	EntityID string
	// Layer restricts validation to this layer. Empty means all layers.
	Layer string
	// IncludeReferences, when true, adds 1-depth references neighbors of
	// covered entities to phase satisfaction closures as advisory members.
	IncludeReferences bool
}

// Validate runs layered validation checks against the graph and returns the
// combined results. The provided context is accepted for forward compatibility
// and is not yet observed.
func (e *Engine) Validate(ctx context.Context, req ValidateRequest) (*validate.ValidateResult, error) {
	_ = ctx

	return readLocked(e, func() (*validate.ValidateResult, error) {
		return e.validateLocked(req)
	})
}

func (e *Engine) validateLocked(req ValidateRequest) (*validate.ValidateResult, error) {
	var phase *string
	if req.Phase != "" {
		p := req.Phase
		phase = &p
	}

	var layer *model.Layer
	if req.Layer != "" {
		switch model.Layer(req.Layer) {
		case model.LayerArch, model.LayerExec, model.LayerMapping:
			l := model.Layer(req.Layer)
			layer = &l
		default:
			return nil, newError(CodeInvalidInput, "layer must be one of: arch, exec, mapping", nil)
		}
	}

	opts := validate.ValidateOptions{
		Checks:            req.Checks,
		Phase:             phase,
		EntityID:          req.EntityID,
		Layer:             layer,
		IncludeReferences: req.IncludeReferences,
	}
	ef := &engineEntityFetcher{idx: e.idx}
	rf := &engineRelationFetcher{idx: e.idx}

	result, err := validate.Validate(opts, rf, ef)
	if err != nil {
		return nil, newError(CodeRuntime, "validate", err)
	}
	return result, nil
}

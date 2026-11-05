package rpc

import (
	"context"
	"encoding/json"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type queryScopeParams struct {
	PhaseID string `json:"phase_id"`
	Layer   string `json:"layer"`
}

func (d *Dispatcher) queryScope(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p queryScopeParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.QueryScope(ctx, specgraph.QueryScopeRequest{
		PhaseID: p.PhaseID,
		Layer:   p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type queryNeighborsParams struct {
	EntityID string `json:"entity_id"`
	Depth    int    `json:"depth"`
}

func (d *Dispatcher) queryNeighbors(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p queryNeighborsParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.QueryNeighbors(ctx, specgraph.QueryNeighborsRequest{
		EntityID: p.EntityID,
		Depth:    p.Depth,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type queryPathParams struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Layer  string `json:"layer"`
}

func (d *Dispatcher) queryPath(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p queryPathParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.QueryPath(ctx, specgraph.QueryPathRequest{
		FromID: p.FromID,
		ToID:   p.ToID,
		Layer:  p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type queryUnresolvedParams struct {
	Type  string `json:"type"`
	Phase string `json:"phase"`
}

func (d *Dispatcher) queryUnresolved(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p queryUnresolvedParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.QueryUnresolved(ctx, specgraph.QueryUnresolvedRequest{Type: p.Type, Phase: p.Phase})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type impactParams struct {
	Sources     []string `json:"sources"`
	Follow      []string `json:"follow"`
	MinSeverity string   `json:"min_severity"`
	Dimension   string   `json:"dimension"`
	Layer       string   `json:"layer"`
}

func (d *Dispatcher) impact(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p impactParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.Impact(ctx, specgraph.ImpactRequest{
		Sources:     p.Sources,
		Follow:      p.Follow,
		MinSeverity: p.MinSeverity,
		Dimension:   p.Dimension,
		Layer:       p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type validateParams struct {
	Checks            []string `json:"checks"`
	Phase             string   `json:"phase"`
	EntityID          string   `json:"entity_id"`
	Layer             string   `json:"layer"`
	IncludeReferences bool     `json:"include_references"`
}

func (d *Dispatcher) validate(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p validateParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.Validate(ctx, specgraph.ValidateRequest{
		Checks:            p.Checks,
		Phase:             p.Phase,
		EntityID:          p.EntityID,
		Layer:             p.Layer,
		IncludeReferences: p.IncludeReferences,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type exportParams struct {
	Format string `json:"format"`
	Center string `json:"center"`
	Depth  int    `json:"depth"`
	Layer  string `json:"layer"`
}

type exportResult struct {
	Format string `json:"format"`
	Data   string `json:"data"`
}

func (d *Dispatcher) export(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p exportParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.Export(ctx, specgraph.ExportRequest{
		Format: p.Format,
		Center: p.Center,
		Depth:  p.Depth,
		Layer:  p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return exportResult{Format: result.Format, Data: result.Data}, nil
}

type phaseNextParams struct {
	Activate bool `json:"activate"`
}

func (d *Dispatcher) phaseNext(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p phaseNextParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	result, err := d.engine.PhaseNext(ctx, specgraph.PhaseNextRequest{Activate: p.Activate})
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

type phaseContextParams struct {
	ID string `json:"id"`
}

func (d *Dispatcher) phaseContext(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p phaseContextParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	if err := model.ValidateEntityID(p.ID, model.EntityTypePhase); err != nil {
		return nil, &rpcError{
			Code:    codeInvalidParams,
			Message: err.Error(),
			Data:    errorData{Code: string(specgraph.CodeInvalidInput), ExitCode: 3},
		}
	}
	result, err := d.engine.PhaseContext(ctx, p.ID)
	if err != nil {
		return nil, engineError(err)
	}
	return result, nil
}

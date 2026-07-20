package rpc

import (
	"context"
	"encoding/json"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type entityCreateParams struct {
	Type        string          `json:"type"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata"`
	Status      string          `json:"status"`
}

func (d *Dispatcher) entityCreate(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p entityCreateParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	entity, err := d.engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type:        p.Type,
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		Metadata:    p.Metadata,
		Status:      p.Status,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.EntityResponse{Entity: entity}, nil
}

type entityGetParams struct {
	ID string `json:"id"`
}

func (d *Dispatcher) entityGet(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p entityGetParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	entity, err := d.engine.GetEntity(ctx, p.ID)
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.EntityResponse{Entity: entity}, nil
}

type entityListParams struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Layer  string `json:"layer"`
}

func (d *Dispatcher) entityList(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p entityListParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	entities, count, err := d.engine.ListEntities(ctx, specgraph.ListEntitiesRequest{
		Type:   p.Type,
		Status: p.Status,
		Layer:  p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.EntityListResponse{Entities: entities, Count: count}, nil
}

// entityUpdateParams mirrors specgraph.UpdateEntityRequest. Pointer fields
// distinguish "unchanged" (absent/null) from "set to value".
type entityUpdateParams struct {
	ID          string           `json:"id"`
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	Status      *string          `json:"status"`
	Metadata    *json.RawMessage `json:"metadata"`
	Force       bool             `json:"force"`
	Reason      string           `json:"reason"`
}

// entityUpdateResult carries the updated entity and, when a gate blocked a
// status transition, the gate report so the client can inspect the blocking
// issues instead of receiving an error.
type entityUpdateResult struct {
	Entity     any  `json:"entity"`
	Blocked    bool `json:"blocked"`
	GateReport any  `json:"gate_report,omitempty"`
}

func (d *Dispatcher) entityUpdate(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p entityUpdateParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	res, err := d.engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		Status:      p.Status,
		Metadata:    p.Metadata,
		Force:       p.Force,
		Reason:      p.Reason,
	})
	if err != nil {
		return nil, engineError(err)
	}
	if res.GateReport != nil {
		return entityUpdateResult{Entity: res.Entity, Blocked: true, GateReport: res.GateReport}, nil
	}
	return entityUpdateResult{Entity: res.Entity, Blocked: false}, nil
}

func (d *Dispatcher) entityDeprecate(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	entity, err := d.engine.DeprecateEntity(ctx, p.ID, p.Reason)
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.EntityResponse{Entity: entity}, nil
}

func (d *Dispatcher) entityDelete(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p entityGetParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	if err := d.engine.DeleteEntity(ctx, p.ID); err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.DeleteResponse{Deleted: p.ID}, nil
}

type relationAddParams struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Type     string          `json:"type"`
	Weight   float64         `json:"weight"`
	Metadata json.RawMessage `json:"metadata"`
}

func (d *Dispatcher) relationAdd(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p relationAddParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	relation, err := d.engine.AddRelation(ctx, specgraph.AddRelationRequest{
		From:     p.From,
		To:       p.To,
		Type:     p.Type,
		Weight:   p.Weight,
		Metadata: p.Metadata,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.RelationResponse{Relation: relation}, nil
}

type relationListParams struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Type  string `json:"type"`
	Layer string `json:"layer"`
}

func (d *Dispatcher) relationList(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p relationListParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	relations, count, err := d.engine.ListRelations(ctx, specgraph.ListRelationsRequest{
		From:  p.From,
		To:    p.To,
		Type:  p.Type,
		Layer: p.Layer,
	})
	if err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.RelationListResponse{Relations: relations, Count: count}, nil
}

type relationDeleteParams struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

func (d *Dispatcher) relationDelete(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p relationDeleteParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}
	if err := d.engine.DeleteRelation(ctx, specgraph.DeleteRelationRequest{
		From: p.From,
		To:   p.To,
		Type: p.Type,
	}); err != nil {
		return nil, engineError(err)
	}
	return jsoncontract.DeleteResponse{Deleted: p.From + "->" + p.To + "[" + p.Type + "]"}, nil
}

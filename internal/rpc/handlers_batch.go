package rpc

import (
	"context"
	"encoding/json"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// batchEntityParams adds a batch-local ref to the entity-create parameters. A
// relation in the same request names an endpoint by ref, which is how a caller
// wires entities whose IDs the engine has not generated yet.
type batchEntityParams struct {
	Ref string `json:"ref"`
	entityCreateParams
}

type batchApplyParams struct {
	Entities  []batchEntityParams `json:"entities"`
	Relations []relationAddParams `json:"relations"`
}

// batchEntityResult reports the generated ID next to the ref that named it.
type batchEntityResult struct {
	Ref    string       `json:"ref,omitempty"`
	Entity model.Entity `json:"entity"`
}

type batchApplyResult struct {
	Entities  []batchEntityResult `json:"entities"`
	Relations []model.Relation    `json:"relations"`
}

func (d *Dispatcher) batchApply(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p batchApplyParams
	if rerr := decodeParams(params, &p); rerr != nil {
		return nil, rerr
	}

	entities := make([]specgraph.BatchEntity, len(p.Entities))
	for i, item := range p.Entities {
		entities[i] = specgraph.BatchEntity{
			Ref: item.Ref,
			CreateEntityRequest: specgraph.CreateEntityRequest{
				Type:        item.Type,
				ID:          item.ID,
				Title:       item.Title,
				Description: item.Description,
				Metadata:    item.Metadata,
				Status:      item.Status,
			},
		}
	}

	relations := make([]specgraph.BatchRelation, len(p.Relations))
	for i, item := range p.Relations {
		relations[i] = specgraph.BatchRelation{
			From:     item.From,
			To:       item.To,
			Type:     item.Type,
			Weight:   item.Weight,
			Metadata: item.Metadata,
		}
	}

	result, err := d.engine.ApplyBatch(ctx, specgraph.BatchRequest{
		Entities:  entities,
		Relations: relations,
	})
	if err != nil {
		return nil, engineError(err)
	}

	created := make([]batchEntityResult, len(result.Entities))
	for i, item := range result.Entities {
		created[i] = batchEntityResult{Ref: item.Ref, Entity: item.Entity}
	}
	return batchApplyResult{Entities: created, Relations: result.Relations}, nil
}

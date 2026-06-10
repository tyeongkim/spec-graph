package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tyeongkim/spec-graph/internal/gate"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// CreateEntityRequest describes the inputs for creating a new entity.
type CreateEntityRequest struct {
	// Type is the entity type (e.g. "requirement", "decision"). Required.
	Type string
	// ID is the entity identifier in PREFIX-NNN form. Optional; when empty it is
	// auto-generated from Type using the next available number for that type.
	ID string
	// Title is the human-readable title. Required.
	Title string
	// Description is an optional longer description.
	Description string
	// Metadata is optional raw JSON stored on the entity.
	Metadata json.RawMessage
	// Status is the initial status. Defaults to "draft" when empty.
	Status string
}

// ListEntitiesRequest describes the optional filters for listing entities.
type ListEntitiesRequest struct {
	// Type filters by entity type. Empty means no type filter.
	Type string
	// Status filters by entity status. Empty means no status filter.
	Status string
	// Layer filters by layer. Empty means no layer filter.
	Layer string
}

// UpdateEntityRequest describes a partial update to an existing entity. Pointer
// fields distinguish "unchanged" (nil) from "set to value" (non-nil).
type UpdateEntityRequest struct {
	// ID identifies the entity to update. Required.
	ID string
	// Title, when non-nil, replaces the entity title.
	Title *string
	// Description, when non-nil, replaces the entity description.
	Description *string
	// Status, when non-nil, replaces the entity status (subject to gate checks).
	Status *string
	// Metadata, when non-nil, replaces the entity metadata.
	Metadata *json.RawMessage
	// Force bypasses gate enforcement on status transitions.
	Force bool
	// Reason is an optional audit note for the change.
	Reason string
}

// UpdateEntityResult is the outcome of an UpdateEntity call. When a status
// transition was blocked by a gate and Force was false, GateReport is non-nil
// and Entity holds the unchanged entity.
type UpdateEntityResult struct {
	// Entity is the updated entity (or the unchanged entity when blocked).
	Entity model.Entity
	// GateReport is non-nil only when the gate blocked the transition and
	// Force was false.
	GateReport *gate.Report
}

// engineEntityFetcher adapts the SQLite index to validate.EntityFetcher.
type engineEntityFetcher struct {
	idx *index.Index
}

// Get returns the entity with the given ID, or a not-found error.
func (f *engineEntityFetcher) Get(id string) (model.Entity, error) {
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
func (f *engineEntityFetcher) List(filters validate.EntityListFilters) ([]model.Entity, error) {
	var ef index.EntityFilters
	if filters.Type != nil {
		ef.Type = string(*filters.Type)
	}
	if filters.Status != nil {
		ef.Status = string(*filters.Status)
	}
	if filters.Layer != nil {
		ef.Layer = string(*filters.Layer)
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

// engineRelationFetcher adapts the SQLite index to validate.RelationFetcher.
type engineRelationFetcher struct {
	idx *index.Index
}

// GetByEntity returns all relations referencing the given entity.
func (f *engineRelationFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	recs, err := f.idx.GetRelationsByEntity(entityID)
	if err != nil {
		return nil, err
	}
	rels := make([]model.Relation, len(recs))
	for i := range recs {
		rec := &recs[i]
		rels[i] = model.Relation{
			FromID:   rec.FromID,
			ToID:     rec.ToID,
			Type:     model.RelationType(rec.Type),
			Layer:    model.Layer(rec.Layer),
			Weight:   rec.Weight,
			Metadata: json.RawMessage(rec.Metadata),
		}
	}
	return rels, nil
}

// engineEntityFromRecord converts an index record into a model.Entity.
func engineEntityFromRecord(rec *index.EntityRecord) model.Entity {
	return model.Entity{
		ID:          rec.ID,
		Type:        model.EntityType(rec.Type),
		Layer:       model.Layer(rec.Layer),
		Status:      model.EntityStatus(rec.Status),
		Title:       rec.Title,
		Description: rec.Description,
		Metadata:    json.RawMessage(rec.Metadata),
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

// CreateEntity registers a new entity in the graph. It validates the type, ID
// format, status, and metadata, rejects duplicates, writes the TOML file, and
// refreshes the index. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) CreateEntity(ctx context.Context, req CreateEntityRequest) (model.Entity, error) {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	if req.Type == "" || req.Title == "" {
		return model.Entity{}, newError(CodeInvalidInput, "type and title are required", nil)
	}

	et := model.EntityType(req.Type)

	id := req.ID
	if id == "" {
		generated, err := e.nextEntityID(et)
		if err != nil {
			return model.Entity{}, err
		}
		id = generated
	} else if err := model.ValidateEntityID(id, et); err != nil {
		return model.Entity{}, newError(CodeInvalidInput, err.Error(), err)
	}

	if e.store.EntityExists(id, et) {
		return model.Entity{}, newError(CodeConflict, fmt.Sprintf("entity %q already exists", id), nil)
	}

	status := model.EntityStatusDraft
	if req.Status != "" {
		status = model.EntityStatus(req.Status)
	}

	schema := spectoml.DefaultSchema()
	if err := schema.ValidateEntity(id, string(et), string(status)); err != nil {
		return model.Entity{}, newError(CodeInvalidInput, err.Error(), err)
	}

	var meta map[string]any
	if len(req.Metadata) > 0 {
		if err := json.Unmarshal(req.Metadata, &meta); err != nil {
			return model.Entity{}, newError(CodeInvalidInput, "metadata must be valid JSON", err)
		}
	}

	now := time.Now()
	ef := &spectoml.EntityFile{
		Schema:      1,
		ID:          id,
		Type:        et,
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    meta,
	}

	if err := e.store.WriteEntity(ef); err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("write entity %q", id), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return model.Entity{}, newError(CodeRuntime, "sync index after create", err)
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

// nextEntityID derives the next available ID for et by scanning existing
// entities of that type in the TOML store (the source of truth). It follows the
// dominant numbering format: if any existing ID is zero-padded it pads new IDs
// to the widest observed width, otherwise it emits unpadded IDs. A collision
// guard increments past any ID already present on disk.
func (e *Engine) nextEntityID(et model.EntityType) (string, error) {
	prefix, ok := model.TypePrefixMap[et]
	if !ok {
		return "", newError(CodeInvalidInput, fmt.Sprintf("unknown entity type %q", et), nil)
	}

	files, err := e.store.ListEntities()
	if err != nil {
		return "", newError(CodeRuntime, "scan existing entities for ID generation", err)
	}

	maxNum := 0
	width := 1
	padded := false
	for i := range files {
		p, num, w, ok := model.ParseEntityID(files[i].ID)
		if !ok || p != prefix {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
		if w > 1 {
			padded = true
			if w > width {
				width = w
			}
		}
	}

	next := maxNum + 1
	for {
		var id string
		if padded {
			id = fmt.Sprintf("%s-%0*d", prefix, width, next)
		} else {
			id = fmt.Sprintf("%s-%d", prefix, next)
		}
		if !e.store.EntityExists(id, et) {
			return id, nil
		}
		next++
	}
}

// GetEntity returns the entity with the given ID. It returns a not-found error
// when no such entity exists. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) GetEntity(ctx context.Context, id string) (model.Entity, error) {
	_ = ctx

	e.mu.RLock()
	defer e.mu.RUnlock()

	rec, err := e.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("get entity %q", id), err)
	}
	if rec == nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
	}

	et := model.EntityType(rec.Type)
	ef, err := e.store.ReadEntity(id, et)
	if err != nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), err)
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

// ListEntities returns entities matching the optional filters in req, along
// with the total count. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) ListEntities(ctx context.Context, req ListEntitiesRequest) ([]model.Entity, int, error) {
	_ = ctx

	e.mu.RLock()
	defer e.mu.RUnlock()

	var filters index.EntityFilters
	if req.Type != "" {
		filters.Type = req.Type
	}
	if req.Status != "" {
		filters.Status = req.Status
	}
	if req.Layer != "" {
		filters.Layer = req.Layer
	}

	records, err := e.idx.ListEntities(filters)
	if err != nil {
		return nil, 0, newError(CodeRuntime, "list entities", err)
	}

	entities := make([]model.Entity, 0, len(records))
	for _, rec := range records {
		et := model.EntityType(rec.Type)
		ef, err := e.store.ReadEntity(rec.ID, et)
		if err != nil {
			return nil, 0, newError(CodeRuntime, fmt.Sprintf("read entity %q", rec.ID), err)
		}
		entity, err := ef.ToEntity()
		if err != nil {
			return nil, 0, newError(CodeRuntime, fmt.Sprintf("convert entity %q", rec.ID), err)
		}
		entities = append(entities, entity)
	}

	return entities, len(entities), nil
}

// UpdateEntity applies a partial update to an existing entity. Pointer fields
// in req distinguish unchanged from explicitly-set values. When the status
// changes, the applicable validation gate is enforced: if the gate blocks the
// transition and Force is false, the entity is left unchanged and the returned
// result carries a non-nil GateReport. The provided context is accepted for
// forward compatibility and is not yet observed.
func (e *Engine) UpdateEntity(ctx context.Context, req UpdateEntityRequest) (UpdateEntityResult, error) {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	rec, err := e.idx.GetEntity(req.ID)
	if err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("get entity %q", req.ID), err)
	}
	if rec == nil {
		return UpdateEntityResult{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.ID), nil)
	}

	et := model.EntityType(rec.Type)
	ef, err := e.store.ReadEntity(req.ID, et)
	if err != nil {
		return UpdateEntityResult{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.ID), err)
	}

	if req.Title != nil {
		ef.Title = *req.Title
	}
	if req.Description != nil {
		ef.Description = *req.Description
	}

	statusChanged := false
	oldStatus := ef.Status
	if req.Status != nil {
		schema := spectoml.DefaultSchema()
		if err := schema.ValidateEntity(ef.ID, string(ef.Type), *req.Status); err != nil {
			return UpdateEntityResult{}, newError(CodeInvalidInput, err.Error(), err)
		}
		ef.Status = model.EntityStatus(*req.Status)
		statusChanged = ef.Status != oldStatus
	}

	if req.Metadata != nil {
		if !json.Valid(*req.Metadata) {
			return UpdateEntityResult{}, newError(CodeInvalidInput, "metadata must be valid JSON", nil)
		}
		var meta map[string]any
		if err := json.Unmarshal(*req.Metadata, &meta); err != nil {
			return UpdateEntityResult{}, newError(CodeInvalidInput, "metadata must be a JSON object", err)
		}
		ef.Metadata = meta
	}

	if statusChanged {
		target := gate.Target{
			EntityID:   req.ID,
			EntityType: ef.Type,
			FromStatus: oldStatus,
			ToStatus:   ef.Status,
		}
		if policy := gate.LookupPolicy(target); policy != nil {
			efAdapter := &engineEntityFetcher{idx: e.idx}
			rfAdapter := &engineRelationFetcher{idx: e.idx}

			report, err := gate.Enforce(target, rfAdapter, efAdapter)
			if err != nil {
				return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("gate enforce %q", req.ID), err)
			}

			if report.Blocked && !req.Force {
				entity, convErr := ef.ToEntity()
				if convErr != nil {
					return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", req.ID), convErr)
				}
				// Reflect the unchanged stored status in the returned entity.
				entity.Status = oldStatus
				return UpdateEntityResult{Entity: entity, GateReport: report}, nil
			}
		}
	}

	ef.UpdatedAt = time.Now()

	if err := e.store.WriteEntity(ef); err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("write entity %q", req.ID), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, "sync index after update", err)
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", req.ID), err)
	}
	return UpdateEntityResult{Entity: entity}, nil
}

// DeprecateEntity sets an entity's status to deprecated, updates its timestamp,
// writes the change, and refreshes the index. The provided context is accepted
// for forward compatibility and is not yet observed.
func (e *Engine) DeprecateEntity(ctx context.Context, id string) (model.Entity, error) {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	rec, err := e.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("get entity %q", id), err)
	}
	if rec == nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
	}

	et := model.EntityType(rec.Type)
	ef, err := e.store.ReadEntity(id, et)
	if err != nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), err)
	}

	ef.Status = model.EntityStatusDeprecated
	ef.UpdatedAt = time.Now()

	if err := e.store.WriteEntity(ef); err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("write entity %q", id), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return model.Entity{}, newError(CodeRuntime, "sync index after deprecate", err)
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

// DeleteEntity removes an entity from the graph. It refuses to delete an entity
// that is still referenced by any relation, and refreshes the index after a
// successful delete. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) DeleteEntity(ctx context.Context, id string) error {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	relations, err := e.idx.GetRelationsByEntity(id)
	if err != nil {
		return newError(CodeRuntime, fmt.Sprintf("check relations for %q", id), err)
	}
	if len(relations) > 0 {
		return newError(
			CodeInvalidInput,
			fmt.Sprintf("cannot delete entity %q: %d relation(s) reference it", id, len(relations)),
			nil,
		)
	}

	rec, err := e.idx.GetEntity(id)
	if err != nil {
		return newError(CodeRuntime, fmt.Sprintf("get entity %q", id), err)
	}
	if rec == nil {
		return newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
	}

	et := model.EntityType(rec.Type)
	if err := e.store.DeleteEntity(id, et); err != nil {
		return newError(CodeRuntime, fmt.Sprintf("delete entity %q", id), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return newError(CodeRuntime, "sync index after delete", err)
	}

	return nil
}

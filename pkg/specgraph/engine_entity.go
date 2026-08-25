package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
	// ID is the entity identifier. Optional; when empty a decentralized,
	// sortable ID of the form PREFIX-<unixSeconds>-<rand3> is generated from
	// Type. When supplied, it must be either that form or the legacy PREFIX-NNN
	// form, and its prefix must match Type.
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

// UpdateOutcome states whether an entity update was persisted or blocked.
type UpdateOutcome string

const (
	UpdateOutcomeApplied          UpdateOutcome = "applied"
	UpdateOutcomeAppliedWithForce UpdateOutcome = "applied_with_force"
	UpdateOutcomeBlocked          UpdateOutcome = "blocked"
)

// UpdateEntityResult is the outcome of an UpdateEntity call. Entity always
// reflects persisted state; GateReport also carries findings accepted by force.
type UpdateEntityResult struct {
	// Entity is the updated entity (or the unchanged entity when blocked).
	Entity model.Entity
	// Outcome distinguishes a normal write, a forced completion write, and no write.
	Outcome UpdateOutcome
	// GateReport is non-nil when a gate blocked the transition or Force bypassed
	// completion findings.
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
	return writeLocked(ctx, e, func() (model.Entity, error) {
		return transact(e, func(tx *txn) (model.Entity, error) {
			return tx.createEntity(req)
		})
	})
}

func (t *txn) createEntity(req CreateEntityRequest) (model.Entity, error) {
	if req.Type == "" || req.Title == "" {
		return model.Entity{}, newError(CodeInvalidInput, "type and title are required", nil)
	}

	et := model.EntityType(req.Type)

	id := req.ID
	if id == "" {
		generated, err := t.nextEntityID(et)
		if err != nil {
			return model.Entity{}, err
		}
		id = generated
	} else if err := model.ValidateEntityID(id, et); err != nil {
		return model.Entity{}, newError(CodeInvalidInput, err.Error(), err)
	}

	if t.exists(id, et) {
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
	if et == model.EntityTypeTask && status != model.EntityStatusDraft {
		return model.Entity{}, newError(CodeInvalidInput, "tasks must be created in draft status", nil)
	}

	var meta map[string]any
	if len(req.Metadata) > 0 {
		if err := json.Unmarshal(req.Metadata, &meta); err != nil {
			return model.Entity{}, newError(CodeInvalidInput, "metadata must be valid JSON", err)
		}
	}
	if et == model.EntityTypeTask {
		if err := validateTaskEntity(req.Title, req.Description, req.Metadata, status); err != nil {
			return model.Entity{}, newError(CodeInvalidInput, err.Error(), err)
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

	if err := t.write(ef); err != nil {
		return model.Entity{}, err
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

// nextEntityID generates a decentralized, sortable entity ID for et via
// model.GenerateEntityID (PREFIX-<unixSeconds>-<rand3>). It retries on the rare
// event that a freshly generated ID already exists, giving a second layer of
// collision protection on top of the create-time existence check.
func (t *txn) nextEntityID(et model.EntityType) (string, error) {
	for i := 0; i < 8; i++ {
		id, err := model.GenerateEntityID(et)
		if err != nil {
			return "", newError(CodeInvalidInput, err.Error(), err)
		}
		if !t.exists(id, et) {
			return id, nil
		}
	}
	return "", newError(CodeRuntime, "failed to generate a unique entity ID after 8 attempts", nil)
}

// GetEntity returns the entity with the given ID. It returns a not-found error
// when no such entity exists. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) GetEntity(ctx context.Context, id string) (model.Entity, error) {
	return readLocked(ctx, e, func() (model.Entity, error) {
		return e.getEntityLocked(id)
	})
}

func (e *Engine) getEntityLocked(id string) (model.Entity, error) {
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
	entities, err := readLocked(ctx, e, func() ([]model.Entity, error) {
		return e.listEntitiesLocked(req)
	})
	if err != nil {
		return nil, 0, err
	}
	return entities, len(entities), nil
}

func (e *Engine) listEntitiesLocked(req ListEntitiesRequest) ([]model.Entity, error) {
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
		return nil, newError(CodeRuntime, "list entities", err)
	}

	entities := make([]model.Entity, 0, len(records))
	for _, rec := range records {
		et := model.EntityType(rec.Type)
		ef, err := e.store.ReadEntity(rec.ID, et)
		if err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("read entity %q", rec.ID), err)
		}
		entity, err := ef.ToEntity()
		if err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("convert entity %q", rec.ID), err)
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// UpdateEntity applies a partial update to an existing entity. Pointer fields
// in req distinguish unchanged from explicitly-set values. When the status
// changes, the applicable validation gate is enforced: if the gate blocks the
// transition and Force is false, the entity is left unchanged and the returned
// result carries a non-nil GateReport.
func (e *Engine) UpdateEntity(ctx context.Context, req UpdateEntityRequest) (UpdateEntityResult, error) {
	return writeLocked(ctx, e, func() (UpdateEntityResult, error) {
		return transact(e, func(tx *txn) (UpdateEntityResult, error) {
			return tx.updateEntity(req)
		})
	})
}

func (t *txn) updateEntity(req UpdateEntityRequest) (UpdateEntityResult, error) {
	existing, err := (&stagedEntityFetcher{tx: t}).Get(req.ID)
	if err != nil {
		return UpdateEntityResult{}, lookupError(fmt.Sprintf("get entity %q", req.ID), req.ID, err)
	}

	ef, err := t.read(req.ID, existing.Type)
	if err != nil {
		return UpdateEntityResult{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.ID), err)
	}
	storedEntity, err := ef.ToEntity()
	if err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert stored entity %q", req.ID), err)
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
		if ef.Type == model.EntityTypeTask && statusChanged {
			if err := model.ValidateTaskTransition(oldStatus, ef.Status); err != nil {
				return UpdateEntityResult{}, newError(CodeInvalidInput, err.Error(), err)
			}
			if ef.Status == model.EntityStatusDeprecated && strings.TrimSpace(req.Reason) == "" {
				return UpdateEntityResult{}, newError(CodeInvalidInput, "task deprecation requires a reason", nil)
			}
		}
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
	if ef.Type == model.EntityTypeTask {
		metadata, err := json.Marshal(ef.Metadata)
		if err != nil {
			return UpdateEntityResult{}, newError(CodeRuntime, "encode task metadata", err)
		}
		if err := validateTaskEntity(ef.Title, ef.Description, metadata, ef.Status); err != nil {
			return UpdateEntityResult{}, newError(CodeInvalidInput, err.Error(), err)
		}
	}

	var gateReport *gate.Report
	outcome := UpdateOutcomeApplied
	if statusChanged {
		candidate, convErr := ef.ToEntity()
		if convErr != nil {
			return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert candidate entity %q", req.ID), convErr)
		}
		target := gate.Target{
			EntityID:   req.ID,
			EntityType: ef.Type,
			FromStatus: oldStatus,
			ToStatus:   ef.Status,
			Candidate:  candidate,
			RepoRoot:   filepath.Dir(t.eng.root),
		}
		if policy := gate.LookupPolicy(target); policy != nil {
			report, err := gate.Enforce(target, &stagedRelationFetcher{tx: t}, &stagedEntityFetcher{tx: t})
			if err != nil {
				return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("gate enforce %q", req.ID), err)
			}

			if report.Blocked && (!req.Force || report.StructuralBlocked) {
				return UpdateEntityResult{Entity: storedEntity, Outcome: UpdateOutcomeBlocked, GateReport: report}, nil
			}
			if report.Blocked && strings.TrimSpace(req.Reason) == "" {
				return UpdateEntityResult{}, newError(CodeInvalidInput, "forced completion requires a reason", nil)
			}
			if report.Blocked {
				gateReport = report
				outcome = UpdateOutcomeAppliedWithForce
				ef.CompletionForced = true
				ef.CompletionReason = strings.TrimSpace(req.Reason)
			}
		}
		if ef.Status == model.EntityStatusResolved && outcome == UpdateOutcomeApplied {
			ef.CompletionForced = false
			ef.CompletionReason = ""
		}
	}

	ef.UpdatedAt = time.Now()

	if err := t.write(ef); err != nil {
		return UpdateEntityResult{}, err
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return UpdateEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", req.ID), err)
	}
	return UpdateEntityResult{Entity: entity, Outcome: outcome, GateReport: gateReport}, nil
}

// DeprecateEntity sets an entity's status to deprecated, updates its timestamp,
// writes the change, and refreshes the index.
func (e *Engine) DeprecateEntity(ctx context.Context, id, reason string) (model.Entity, error) {
	return writeLocked(ctx, e, func() (model.Entity, error) {
		return transact(e, func(tx *txn) (model.Entity, error) {
			return tx.deprecateEntity(id, reason)
		})
	})
}

func (t *txn) deprecateEntity(id, reason string) (model.Entity, error) {
	existing, err := (&stagedEntityFetcher{tx: t}).Get(id)
	if err != nil {
		return model.Entity{}, lookupError(fmt.Sprintf("get entity %q", id), id, err)
	}

	if existing.Type == model.EntityTypeTask {
		status := string(model.EntityStatusDeprecated)
		result, updateErr := t.updateEntity(UpdateEntityRequest{ID: id, Status: &status, Reason: reason})
		if updateErr != nil {
			return model.Entity{}, updateErr
		}
		return result.Entity, nil
	}

	ef, err := t.read(id, existing.Type)
	if err != nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), err)
	}

	ef.Status = model.EntityStatusDeprecated
	ef.UpdatedAt = time.Now()

	if err := t.write(ef); err != nil {
		return model.Entity{}, err
	}

	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

func validateTaskEntity(title, description string, metadata json.RawMessage, status model.EntityStatus) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("task title must be non-empty")
	}
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("task description must be non-empty")
	}
	if _, err := model.DecodeTaskContract(metadata, status); err != nil {
		return err
	}
	return nil
}

// ImportEntitiesRequest describes a bulk entity creation.
type ImportEntitiesRequest struct {
	Entities []CreateEntityRequest
}

// ImportEntitiesResult reports which entities were created and which were
// skipped because they already existed.
type ImportEntitiesResult struct {
	Created []string
	Skipped []BootstrapSkippedItem
}

// ImportEntities creates every entity in req as one unit. An entity that
// already exists is skipped, keeping a re-run of the same input idempotent; any
// other problem aborts the import and leaves the graph untouched.
func (e *Engine) ImportEntities(ctx context.Context, req ImportEntitiesRequest) (ImportEntitiesResult, error) {
	return writeLocked(ctx, e, func() (ImportEntitiesResult, error) {
		return transact(e, func(tx *txn) (ImportEntitiesResult, error) {
			return tx.importEntities(req)
		})
	})
}

func (t *txn) importEntities(req ImportEntitiesRequest) (ImportEntitiesResult, error) {
	var result ImportEntitiesResult

	for _, item := range req.Entities {
		created, err := t.createEntity(item)
		if err != nil {
			if IsConflict(err) {
				result.Skipped = append(result.Skipped, BootstrapSkippedItem{
					ID: item.ID, Reason: "already exists",
				})
				continue
			}
			return ImportEntitiesResult{}, err
		}
		result.Created = append(result.Created, created.ID)
	}

	return result, nil
}

// DeleteEntity removes an entity from the graph. It refuses to delete an entity
// that is still referenced by any relation, and refreshes the index after a
// successful delete. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) DeleteEntity(ctx context.Context, id string) error {
	return writeLockedErr(ctx, e, func() error {
		return transactErr(e, func(tx *txn) error {
			return tx.deleteEntity(id)
		})
	})
}

func (t *txn) deleteEntity(id string) error {
	relations, err := (&stagedRelationFetcher{tx: t}).GetByEntity(id)
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

	entity, err := (&stagedEntityFetcher{tx: t}).Get(id)
	if err != nil {
		return lookupError(fmt.Sprintf("get entity %q", id), id, err)
	}

	return t.remove(id, entity.Type)
}

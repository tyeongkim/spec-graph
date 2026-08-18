package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

// ReviseEntityRequest describes a new revision of an existing arch entity.
// Nil Title, Description, and Metadata carry the superseded value forward.
type ReviseEntityRequest struct {
	// ID identifies the entity being superseded. Required.
	ID string
	// Title, when non-nil, replaces the title on the new revision.
	Title *string
	// Description, when non-nil, replaces the description on the new revision.
	Description *string
	// Metadata, when non-nil, replaces the metadata on the new revision.
	Metadata *json.RawMessage
	// Reason records why the entity was revised. Required.
	Reason string
}

// ReviseEntityResult reports the created revision and how inbound relations moved.
type ReviseEntityResult struct {
	// Revision is the newly created entity carrying the supersedes edge.
	Revision model.Entity
	// Superseded is the prior entity, now deprecated.
	Superseded model.Entity
	// CarriedRelations are inbound relations repointed onto Revision.
	CarriedRelations []model.Relation
	// RetainedRelations are inbound relations deliberately left on Superseded.
	RetainedRelations []model.Relation
}

// ReviseEntity supersedes an arch entity with a new revision in one atomic
// operation: it creates the revision carrying the prior outbound relations, adds
// a supersedes edge whose metadata records Reason, repoints inbound
// relations onto the revision, and deprecates the prior entity.
//
// Mapping relations from a resolved phase or task stay on the superseded entity,
// because repointing them would attribute delivery of the revision to execution
// that completed before the revision existed.
func (e *Engine) ReviseEntity(ctx context.Context, req ReviseEntityRequest) (ReviseEntityResult, error) {
	_ = ctx

	return writeLocked(e, func() (ReviseEntityResult, error) {
		return e.reviseEntityLocked(req)
	})
}

func (e *Engine) reviseEntityLocked(req ReviseEntityRequest) (ReviseEntityResult, error) {
	if req.ID == "" {
		return ReviseEntityResult{}, newError(CodeInvalidInput, "id is required", nil)
	}
	if req.Reason == "" {
		return ReviseEntityResult{}, newError(CodeInvalidInput, "reason is required", nil)
	}

	priorRec, err := e.idx.GetEntity(req.ID)
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("lookup entity %q", req.ID), err)
	}
	if priorRec == nil {
		return ReviseEntityResult{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.ID), nil)
	}

	entityType := model.EntityType(priorRec.Type)
	if model.LayerForEntityType(entityType) != model.LayerArch {
		return ReviseEntityResult{}, newError(CodeInvalidInput, fmt.Sprintf("entity %q is not an arch entity; only arch entities form revision chains", req.ID), nil)
	}

	prior, err := e.store.ReadEntity(req.ID, entityType)
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("read entity %q", req.ID), err)
	}

	inbound, err := e.inboundRelations(req.ID)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	if successor := findSuccessor(inbound); successor != "" {
		return ReviseEntityResult{}, newError(CodeConflict, fmt.Sprintf("entity %q is already superseded by %q; revise %q instead", req.ID, successor, successor), nil)
	}
	if prior.Status == model.EntityStatusDeprecated {
		return ReviseEntityResult{}, newError(CodeInvalidInput, fmt.Sprintf("entity %q is deprecated; revision chains start from a live entity", req.ID), nil)
	}

	revisionID, err := e.nextEntityID(entityType)
	if err != nil {
		return ReviseEntityResult{}, err
	}

	carried, retained, err := e.partitionInbound(inbound)
	if err != nil {
		return ReviseEntityResult{}, err
	}

	revision, err := buildRevision(prior, revisionID, req)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	if err := e.store.WriteEntity(revision); err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("write revision %q", revisionID), err)
	}
	if _, err := e.syncer.EnsureFresh(); err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, "sync index after writing revision", err)
	}

	if err := e.moveRelations(prior, carried, revisionID); err != nil {
		return ReviseEntityResult{}, err
	}

	prior, err = e.store.ReadEntity(req.ID, entityType)
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("reread entity %q", req.ID), err)
	}
	prior.Status = model.EntityStatusDeprecated
	prior.UpdatedAt = time.Now()
	if err := e.store.WriteEntity(prior); err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("deprecate entity %q", req.ID), err)
	}
	if _, err := e.syncer.EnsureFresh(); err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, "sync index after revise", err)
	}

	revisionEntity, err := e.entityByID(revisionID)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	priorEntity, err := prior.ToEntity()
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", req.ID), err)
	}

	return ReviseEntityResult{
		Revision:          revisionEntity,
		Superseded:        priorEntity,
		CarriedRelations:  carried,
		RetainedRelations: retained,
	}, nil
}

// buildRevision produces the revision file: the supersedes edge plus the prior
// entity's outbound relations, with request fields overriding carried values.
// Symmetric relations are excluded because their owning file depends on the new
// ID; moveRelations re-adds them through the normalizing relation path.
func buildRevision(prior *spectoml.EntityFile, revisionID string, req ReviseEntityRequest) (*spectoml.EntityFile, error) {
	metadata := prior.Metadata
	if req.Metadata != nil {
		if !json.Valid(*req.Metadata) {
			return nil, newError(CodeInvalidInput, "metadata must be valid JSON", nil)
		}
		var replacement map[string]any
		if err := json.Unmarshal(*req.Metadata, &replacement); err != nil {
			return nil, newError(CodeInvalidInput, "metadata must be a JSON object", err)
		}
		metadata = replacement
	}

	title := prior.Title
	if req.Title != nil {
		title = *req.Title
	}
	description := prior.Description
	if req.Description != nil {
		description = *req.Description
	}

	supersedes := spectoml.RelationEntry{
		To:       prior.ID,
		Type:     model.RelationSupersedes,
		Metadata: map[string]any{"reason": req.Reason},
	}

	relations := []spectoml.RelationEntry{supersedes}
	for _, entry := range prior.Relations {
		if isSymmetricRelation(entry.Type) {
			continue
		}
		relations = append(relations, entry)
	}

	now := time.Now()
	return &spectoml.EntityFile{
		Schema:      prior.Schema,
		ID:          revisionID,
		Type:        prior.Type,
		Title:       title,
		Description: description,
		Status:      model.EntityStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
		Relations:   relations,
	}, nil
}

// moveRelations repoints carried inbound relations onto the revision and re-adds
// the prior entity's symmetric relations from the revision. Both go through
// addRelationLocked and deleteRelationLocked so edge-matrix validation and
// symmetric-owner normalization stay in one place.
func (e *Engine) moveRelations(prior *spectoml.EntityFile, carried []model.Relation, revisionID string) error {
	for _, entry := range prior.Relations {
		if !isSymmetricRelation(entry.Type) {
			continue
		}
		if err := e.transferRelation(prior.ID, entry.To, revisionID, entry.To, entry.Type, entry.Weight, entry.Metadata); err != nil {
			return err
		}
	}

	for _, relation := range carried {
		from := relation.FromID
		to := revisionID
		if isSymmetricRelation(relation.Type) {
			from = revisionID
			to = relation.FromID
		}
		var metadata map[string]any
		if len(relation.Metadata) > 0 {
			if err := json.Unmarshal(relation.Metadata, &metadata); err != nil {
				return newError(CodeRuntime, fmt.Sprintf("decode metadata of %q relation from %q", relation.Type, relation.FromID), err)
			}
		}
		if err := e.transferRelation(relation.FromID, prior.ID, from, to, relation.Type, relation.Weight, metadata); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) transferRelation(oldFrom, oldTo, newFrom, newTo string, relationType model.RelationType, weight float64, metadata map[string]any) error {
	var metadataJSON json.RawMessage
	if len(metadata) > 0 {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return newError(CodeRuntime, fmt.Sprintf("encode metadata of %q relation", relationType), err)
		}
		metadataJSON = encoded
	}

	if _, err := e.addRelationLocked(AddRelationRequest{
		From:     newFrom,
		To:       newTo,
		Type:     string(relationType),
		Weight:   weight,
		Metadata: metadataJSON,
	}); err != nil {
		return err
	}

	return e.deleteRelationLocked(DeleteRelationRequest{
		From: oldFrom,
		To:   oldTo,
		Type: string(relationType),
	})
}

func (e *Engine) inboundRelations(id string) ([]model.Relation, error) {
	records, err := e.idx.GetRelationsByEntity(id)
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("lookup relations for %q", id), err)
	}

	var inbound []model.Relation
	for _, record := range records {
		if record.ToID != id {
			continue
		}
		relationType := model.RelationType(record.Type)
		inbound = append(inbound, model.Relation{
			FromID:   record.FromID,
			ToID:     record.ToID,
			Type:     relationType,
			Layer:    model.LayerForRelationType(relationType),
			Weight:   record.Weight,
			Metadata: json.RawMessage(record.Metadata),
		})
	}
	return inbound, nil
}

// findSuccessor reports the entity that already supersedes the revised entity,
// guarding against a forked chain with no single latest revision.
func findSuccessor(inbound []model.Relation) string {
	for _, relation := range inbound {
		if relation.Type == model.RelationSupersedes {
			return relation.FromID
		}
	}
	return ""
}

// partitionInbound splits relations pointing at the revised entity into those
// that move onto the revision and those that stay as a record of completed
// execution.
func (e *Engine) partitionInbound(inbound []model.Relation) (carried, retained []model.Relation, err error) {
	for _, relation := range inbound {
		if relation.Type == model.RelationSupersedes {
			continue
		}
		completed, completedErr := e.recordsCompletedExecution(relation)
		if completedErr != nil {
			return nil, nil, completedErr
		}
		if completed {
			retained = append(retained, relation)
			continue
		}
		carried = append(carried, relation)
	}
	return carried, retained, nil
}

func (e *Engine) recordsCompletedExecution(relation model.Relation) (bool, error) {
	if relation.Layer != model.LayerMapping {
		return false, nil
	}
	source, err := e.idx.GetEntity(relation.FromID)
	if err != nil {
		return false, newError(CodeRuntime, fmt.Sprintf("lookup entity %q", relation.FromID), err)
	}
	if source == nil {
		return false, nil
	}
	return model.EntityStatus(source.Status) == model.EntityStatusResolved, nil
}

func (e *Engine) entityByID(id string) (model.Entity, error) {
	record, err := e.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("lookup entity %q", id), err)
	}
	if record == nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
	}

	ef, err := e.store.ReadEntity(id, model.EntityType(record.Type))
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("read entity %q", id), err)
	}
	entity, err := ef.ToEntity()
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", id), err)
	}
	return entity, nil
}

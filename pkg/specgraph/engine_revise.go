package specgraph

import (
	"context"
	"encoding/json"
	"errors"
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
	return writeLocked(ctx, e, func() (ReviseEntityResult, error) {
		return transact(e, func(tx *txn) (ReviseEntityResult, error) {
			return tx.reviseEntity(req)
		})
	})
}

func (t *txn) reviseEntity(req ReviseEntityRequest) (ReviseEntityResult, error) {
	if req.ID == "" {
		return ReviseEntityResult{}, newError(CodeInvalidInput, "id is required", nil)
	}
	if req.Reason == "" {
		return ReviseEntityResult{}, newError(CodeInvalidInput, "reason is required", nil)
	}

	priorEntity, err := (&stagedEntityFetcher{tx: t}).Get(req.ID)
	if err != nil {
		return ReviseEntityResult{}, lookupError(fmt.Sprintf("lookup entity %q", req.ID), req.ID, err)
	}

	entityType := priorEntity.Type
	if model.LayerForEntityType(entityType) != model.LayerArch {
		return ReviseEntityResult{}, newError(CodeInvalidInput, fmt.Sprintf("entity %q is not an arch entity; only arch entities form revision chains", req.ID), nil)
	}

	prior, err := t.read(req.ID, entityType)
	if err != nil {
		return ReviseEntityResult{}, err
	}

	inbound, err := t.inboundRelations(req.ID)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	if successor := findSuccessor(inbound); successor != "" {
		return ReviseEntityResult{}, newError(CodeConflict, fmt.Sprintf("entity %q is already superseded by %q; revise %q instead", req.ID, successor, successor), nil)
	}
	if prior.Status == model.EntityStatusDeprecated {
		return ReviseEntityResult{}, newError(CodeInvalidInput, fmt.Sprintf("entity %q is deprecated; revision chains start from a live entity", req.ID), nil)
	}

	revisionID, err := t.nextEntityID(entityType)
	if err != nil {
		return ReviseEntityResult{}, err
	}

	carried, retained, err := t.partitionInbound(inbound)
	if err != nil {
		return ReviseEntityResult{}, err
	}

	revision, err := buildRevision(prior, revisionID, req)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	if err := t.write(revision); err != nil {
		return ReviseEntityResult{}, err
	}

	if err := t.moveRelations(prior, carried, revisionID); err != nil {
		return ReviseEntityResult{}, err
	}

	superseded, err := t.read(req.ID, entityType)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	superseded.Status = model.EntityStatusDeprecated
	superseded.UpdatedAt = time.Now()
	if err := t.write(superseded); err != nil {
		return ReviseEntityResult{}, err
	}

	revisionFile, err := t.read(revisionID, entityType)
	if err != nil {
		return ReviseEntityResult{}, err
	}
	revisionEntity, err := revisionFile.ToEntity()
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", revisionID), err)
	}
	supersededEntity, err := superseded.ToEntity()
	if err != nil {
		return ReviseEntityResult{}, newError(CodeRuntime, fmt.Sprintf("convert entity %q", req.ID), err)
	}

	return ReviseEntityResult{
		Revision:          revisionEntity,
		Superseded:        supersededEntity,
		CarriedRelations:  carried,
		RetainedRelations: retained,
	}, nil
}

// buildRevision produces the revision file: the supersedes edge plus the prior
// entity's outbound relations, with request fields overriding carried values.
// Symmetric relations are excluded because their owning file depends on the new
// ID; moveRelations re-adds them through the normalizing relation path.
func buildRevision(prior *spectoml.EntityFile, revisionID string, req ReviseEntityRequest) (*spectoml.EntityFile, error) {
	carried := prior.Clone()

	metadata := carried.Metadata
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

	title := carried.Title
	if req.Title != nil {
		title = *req.Title
	}
	description := carried.Description
	if req.Description != nil {
		description = *req.Description
	}

	supersedes := spectoml.RelationEntry{
		To:       prior.ID,
		Type:     model.RelationSupersedes,
		Metadata: map[string]any{"reason": req.Reason},
	}

	relations := []spectoml.RelationEntry{supersedes}
	for _, entry := range carried.Relations {
		if spectoml.IsSymmetricRelation(entry.Type) {
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
// addRelation and deleteRelation so edge-matrix validation and symmetric-owner
// normalization stay in one place.
func (t *txn) moveRelations(prior *spectoml.EntityFile, carried []model.Relation, revisionID string) error {
	for _, entry := range prior.Relations {
		if !spectoml.IsSymmetricRelation(entry.Type) {
			continue
		}
		if err := t.transferRelation(prior.ID, entry.To, revisionID, entry.To, entry.Type, entry.Weight, entry.Metadata); err != nil {
			return err
		}
	}

	for _, relation := range carried {
		from := relation.FromID
		to := revisionID
		if spectoml.IsSymmetricRelation(relation.Type) {
			from = revisionID
			to = relation.FromID
		}
		var metadata map[string]any
		if len(relation.Metadata) > 0 {
			if err := json.Unmarshal(relation.Metadata, &metadata); err != nil {
				return newError(CodeRuntime, fmt.Sprintf("decode metadata of %q relation from %q", relation.Type, relation.FromID), err)
			}
		}
		if err := t.transferRelation(relation.FromID, prior.ID, from, to, relation.Type, relation.Weight, metadata); err != nil {
			return err
		}
	}

	return nil
}

func (t *txn) transferRelation(oldFrom, oldTo, newFrom, newTo string, relationType model.RelationType, weight float64, metadata map[string]any) error {
	var metadataJSON json.RawMessage
	if len(metadata) > 0 {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return newError(CodeRuntime, fmt.Sprintf("encode metadata of %q relation", relationType), err)
		}
		metadataJSON = encoded
	}

	if _, err := t.addRelation(AddRelationRequest{
		From:     newFrom,
		To:       newTo,
		Type:     string(relationType),
		Weight:   weight,
		Metadata: metadataJSON,
	}); err != nil {
		return err
	}

	return t.deleteRelation(DeleteRelationRequest{
		From: oldFrom,
		To:   oldTo,
		Type: string(relationType),
	})
}

func (t *txn) inboundRelations(id string) ([]model.Relation, error) {
	relations, err := (&stagedRelationFetcher{tx: t}).GetByEntity(id)
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("lookup relations for %q", id), err)
	}

	var inbound []model.Relation
	for _, relation := range relations {
		if relation.ToID != id {
			continue
		}
		inbound = append(inbound, relation)
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
func (t *txn) partitionInbound(inbound []model.Relation) (carried, retained []model.Relation, err error) {
	for _, relation := range inbound {
		if relation.Type == model.RelationSupersedes {
			continue
		}
		completed, completedErr := t.recordsCompletedExecution(relation)
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

func (t *txn) recordsCompletedExecution(relation model.Relation) (bool, error) {
	if relation.Layer != model.LayerMapping {
		return false, nil
	}
	source, err := (&stagedEntityFetcher{tx: t}).Get(relation.FromID)
	if err != nil {
		var missing *model.ErrEntityNotFound
		if errors.As(err, &missing) {
			return false, nil
		}
		return false, newError(CodeRuntime, fmt.Sprintf("lookup entity %q", relation.FromID), err)
	}
	return source.Status == model.EntityStatusResolved, nil
}

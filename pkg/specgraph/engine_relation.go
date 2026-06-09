package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

// AddRelationRequest describes inputs for adding a relation.
type AddRelationRequest struct {
	From     string          // source entity ID (required)
	To       string          // target entity ID (required)
	Type     string          // relation type (required)
	Weight   float64         // relation weight (default 1.0)
	Metadata json.RawMessage // optional metadata JSON
}

// ListRelationsRequest describes filters for listing relations.
type ListRelationsRequest struct {
	From  string // filter by source entity ID
	To    string // filter by target entity ID
	Type  string // filter by relation type
	Layer string // filter by layer (arch/exec/mapping)
}

// DeleteRelationRequest describes inputs for deleting a relation.
type DeleteRelationRequest struct {
	From string // source entity ID (required)
	To   string // target entity ID (required)
	Type string // relation type (required)
}

// isSymmetricRelation reports whether the relation type is symmetric, meaning
// the relation is owned by the lexicographically smaller endpoint regardless of
// the requested direction.
func isSymmetricRelation(rt model.RelationType) bool {
	return rt == model.RelationConflictsWith
}

// isValidRelationType reports whether rt is one of the known relation types.
func isValidRelationType(rt model.RelationType) bool {
	for _, valid := range model.ValidRelationTypes {
		if valid == rt {
			return true
		}
	}
	return false
}

// AddRelation registers a new relation between two existing entities. It
// validates the relation type, rejects self-loops, verifies both endpoints
// exist, enforces the edge matrix, normalizes symmetric relations to a
// canonical owner, rejects duplicates, and writes the owning entity's TOML
// file. When the relation type is "delivers", a draft target entity is
// auto-activated. The index is refreshed afterward. The provided context is
// accepted for forward compatibility and is not yet observed.
func (e *Engine) AddRelation(ctx context.Context, req AddRelationRequest) (model.Relation, error) {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	if req.From == "" || req.To == "" || req.Type == "" {
		return model.Relation{}, newError(CodeInvalidInput, "from, to, and type are required", nil)
	}

	rt := model.RelationType(req.Type)
	if !isValidRelationType(rt) {
		return model.Relation{}, newError(CodeInvalidInput, fmt.Sprintf("unknown relation type %q", req.Type), nil)
	}

	if req.From == req.To {
		return model.Relation{}, newError(CodeInvalidInput, "self-loop not allowed", nil)
	}

	fromRec, err := e.idx.GetEntity(req.From)
	if err != nil {
		return model.Relation{}, newError(CodeRuntime, fmt.Sprintf("lookup from entity %q", req.From), err)
	}
	if fromRec == nil {
		return model.Relation{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.From), nil)
	}

	toRec, err := e.idx.GetEntity(req.To)
	if err != nil {
		return model.Relation{}, newError(CodeRuntime, fmt.Sprintf("lookup to entity %q", req.To), err)
	}
	if toRec == nil {
		return model.Relation{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", req.To), nil)
	}

	fromType := model.EntityType(fromRec.Type)
	toType := model.EntityType(toRec.Type)
	if !model.IsEdgeAllowed(rt, fromType, toType, nil) {
		return model.Relation{}, newError(
			CodeInvalidInput,
			"relation type not allowed between these entity types",
			&model.ErrInvalidEdge{FromType: fromType, ToType: toType, RelationType: rt},
		)
	}

	var relMeta map[string]any
	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return model.Relation{}, newError(CodeInvalidInput, "metadata must be valid JSON", nil)
		}
		if err := json.Unmarshal(req.Metadata, &relMeta); err != nil {
			return model.Relation{}, newError(CodeInvalidInput, "metadata must be a JSON object", err)
		}
	}

	ownerID := req.From
	ownerType := fromType
	targetID := req.To
	if isSymmetricRelation(rt) && req.From > req.To {
		ownerID = req.To
		ownerType = toType
		targetID = req.From
	}

	ef, err := e.store.ReadEntity(ownerID, ownerType)
	if err != nil {
		return model.Relation{}, newError(CodeRuntime, fmt.Sprintf("read owner entity %q", ownerID), err)
	}

	for _, existing := range ef.Relations {
		if existing.To == targetID && existing.Type == rt {
			return model.Relation{}, newError(
				CodeConflict,
				fmt.Sprintf("relation %q from %q to %q already exists", rt, req.From, req.To),
				&model.ErrDuplicateRelation{FromID: req.From, ToID: req.To, RelationType: rt},
			)
		}
	}

	relWeight := req.Weight
	if relWeight == 1.0 {
		relWeight = 0
	}

	ef.Relations = append(ef.Relations, spectoml.RelationEntry{
		To:       targetID,
		Type:     rt,
		Weight:   relWeight,
		Metadata: relMeta,
	})

	if err := e.store.WriteEntity(ef); err != nil {
		return model.Relation{}, newError(CodeRuntime, fmt.Sprintf("write entity %q", ownerID), err)
	}

	if rt == model.RelationDelivers {
		if err := e.autoActivateOnDelivers(req.To, toType); err != nil {
			return model.Relation{}, newError(CodeRuntime, fmt.Sprintf("auto-activate %q", req.To), err)
		}
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return model.Relation{}, newError(CodeRuntime, "sync index after add relation", err)
	}

	var metaJSON json.RawMessage
	if len(relMeta) > 0 {
		metaJSON, _ = json.Marshal(relMeta)
	}

	return model.Relation{
		FromID:   req.From,
		ToID:     req.To,
		Type:     rt,
		Layer:    model.LayerForRelationType(rt),
		Weight:   req.Weight,
		Metadata: metaJSON,
	}, nil
}

// ListRelations returns relations matching the optional filters in req, along
// with the total count. The provided context is accepted for forward
// compatibility and is not yet observed.
func (e *Engine) ListRelations(ctx context.Context, req ListRelationsRequest) ([]model.Relation, int, error) {
	_ = ctx

	e.mu.RLock()
	defer e.mu.RUnlock()

	var filters index.RelationFilters
	if req.From != "" {
		filters.FromID = req.From
	}
	if req.To != "" {
		filters.ToID = req.To
	}
	if req.Type != "" {
		filters.Type = req.Type
	}
	if req.Layer != "" {
		filters.Layer = req.Layer
	}

	records, err := e.idx.ListRelations(filters)
	if err != nil {
		return nil, 0, newError(CodeRuntime, "list relations", err)
	}

	relations := make([]model.Relation, 0, len(records))
	for _, rec := range records {
		relations = append(relations, model.Relation{
			FromID:   rec.FromID,
			ToID:     rec.ToID,
			Type:     model.RelationType(rec.Type),
			Layer:    model.Layer(rec.Layer),
			Weight:   rec.Weight,
			Metadata: json.RawMessage(rec.Metadata),
		})
	}

	return relations, len(relations), nil
}

// DeleteRelation removes a relation between two entities. It validates the
// required fields, normalizes symmetric relations to their canonical owner,
// reads the owning entity, removes the matching relation entry (returning a
// not-found error when no match exists), writes the change, and refreshes the
// index. The provided context is accepted for forward compatibility and is not
// yet observed.
func (e *Engine) DeleteRelation(ctx context.Context, req DeleteRelationRequest) error {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	if req.From == "" || req.To == "" || req.Type == "" {
		return newError(CodeInvalidInput, "from, to, and type are required", nil)
	}

	rt := model.RelationType(req.Type)

	ownerID := req.From
	targetID := req.To
	if isSymmetricRelation(rt) && req.From > req.To {
		ownerID = req.To
		targetID = req.From
	}

	ownerRec, err := e.idx.GetEntity(ownerID)
	if err != nil {
		return newError(CodeRuntime, fmt.Sprintf("lookup owner entity %q", ownerID), err)
	}
	if ownerRec == nil {
		return newError(CodeNotFound, fmt.Sprintf("entity %q not found", ownerID), nil)
	}

	ownerType := model.EntityType(ownerRec.Type)
	ef, err := e.store.ReadEntity(ownerID, ownerType)
	if err != nil {
		return newError(CodeRuntime, fmt.Sprintf("read owner entity %q", ownerID), err)
	}

	found := false
	filtered := make([]spectoml.RelationEntry, 0, len(ef.Relations))
	for _, rel := range ef.Relations {
		if rel.To == targetID && rel.Type == rt {
			found = true
			continue
		}
		filtered = append(filtered, rel)
	}

	if !found {
		return newError(
			CodeNotFound,
			"relation not found",
			&model.ErrRelationNotFound{Key: fmt.Sprintf("%s->%s[%s]", req.From, req.To, req.Type)},
		)
	}

	ef.Relations = filtered

	if err := e.store.WriteEntity(ef); err != nil {
		return newError(CodeRuntime, fmt.Sprintf("write entity %q", ownerID), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return newError(CodeRuntime, "sync index after delete relation", err)
	}

	return nil
}

// autoActivateOnDelivers transitions a draft target entity to active when a
// "delivers" relation is added to it. Non-draft targets are left unchanged.
func (e *Engine) autoActivateOnDelivers(entityID string, entityType model.EntityType) error {
	targetEF, err := e.store.ReadEntity(entityID, entityType)
	if err != nil {
		return fmt.Errorf("read target entity %q: %w", entityID, err)
	}
	if targetEF.Status != model.EntityStatusDraft {
		return nil
	}

	targetEF.Status = model.EntityStatusActive
	targetEF.UpdatedAt = time.Now()
	if err := e.store.WriteEntity(targetEF); err != nil {
		return fmt.Errorf("write target entity %q: %w", entityID, err)
	}
	return nil
}

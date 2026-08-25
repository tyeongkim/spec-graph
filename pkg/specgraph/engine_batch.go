package specgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// BatchEntity is one entity to create within a batch. Ref names the entity for
// the duration of the batch so relations in the same request can reference an ID
// that does not exist yet; it is not persisted. Ref is optional, and an entity
// declared without one cannot be referenced.
type BatchEntity struct {
	Ref string
	CreateEntityRequest
}

// BatchRelation is one relation to add within a batch. From and To each hold
// either a ref declared by an entity in the same request or the ID of an entity
// that already exists in the graph.
type BatchRelation struct {
	From     string
	To       string
	Type     string
	Weight   float64
	Metadata json.RawMessage
}

// BatchRequest describes entities and relations to commit as one unit. Every
// entity is created before any relation, so ordering matters only among
// relations.
type BatchRequest struct {
	Entities  []BatchEntity
	Relations []BatchRelation
}

// BatchEntityResult pairs a created entity with the ref that named it, so a
// caller that omitted IDs can resolve the generated ones back to its own input.
type BatchEntityResult struct {
	Ref    string
	Entity model.Entity
}

// BatchResult holds the entities and relations a batch created, in request order.
type BatchResult struct {
	Entities  []BatchEntityResult
	Relations []model.Relation
}

// ApplyBatch creates every entity and relation in req as one unit. Any failure
// aborts the batch and leaves the graph untouched: unlike ImportEntities and
// BootstrapImport nothing is skipped, because a batch describes one intended
// graph shape rather than a set of independently useful candidates.
//
// Each item goes through the same validation as its single-item counterpart, so
// edge-matrix rules, symmetric-relation ownership, phase scope, and
// delivers-driven activation all hold within a batch. Failures name the
// position of the offending item.
func (e *Engine) ApplyBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	return writeLocked(ctx, e, func() (BatchResult, error) {
		return transact(e, func(tx *txn) (BatchResult, error) {
			return tx.applyBatch(req)
		})
	})
}

func (t *txn) applyBatch(req BatchRequest) (BatchResult, error) {
	if len(req.Entities) == 0 && len(req.Relations) == 0 {
		return BatchResult{}, newError(CodeInvalidInput, "batch must contain at least one entity or relation", nil)
	}

	result := BatchResult{
		Entities:  make([]BatchEntityResult, 0, len(req.Entities)),
		Relations: make([]model.Relation, 0, len(req.Relations)),
	}
	refToID := make(map[string]string, len(req.Entities))

	for i, item := range req.Entities {
		subject := fmt.Sprintf("entity %d", i)
		if item.Ref != "" {
			subject = fmt.Sprintf("entity %d (ref %q)", i, item.Ref)
		}

		if err := validateRef(item.Ref, refToID); err != nil {
			return BatchResult{}, batchError(subject, err)
		}

		entity, err := t.createEntity(item.CreateEntityRequest)
		if err != nil {
			return BatchResult{}, batchError(subject, err)
		}

		if item.Ref != "" {
			refToID[item.Ref] = entity.ID
		}
		result.Entities = append(result.Entities, BatchEntityResult{Ref: item.Ref, Entity: entity})
	}

	for i, item := range req.Relations {
		subject := fmt.Sprintf("relation %d", i)

		from, err := resolveEndpoint(item.From, "from", refToID)
		if err != nil {
			return BatchResult{}, batchError(subject, err)
		}
		to, err := resolveEndpoint(item.To, "to", refToID)
		if err != nil {
			return BatchResult{}, batchError(subject, err)
		}

		relation, err := t.addRelation(AddRelationRequest{
			From:     from,
			To:       to,
			Type:     item.Type,
			Weight:   item.Weight,
			Metadata: item.Metadata,
		})
		if err != nil {
			return BatchResult{}, batchError(subject, err)
		}
		result.Relations = append(result.Relations, relation)
	}

	return result, nil
}

// validateRef rejects a ref shaped like an entity ID, which is what keeps refs
// and real IDs in separate namespaces without reserving a sigil for either.
func validateRef(ref string, declared map[string]string) error {
	if ref == "" {
		return nil
	}
	if _, _, _, looksLikeEntityID := model.ParseEntityID(ref); looksLikeEntityID {
		return newError(CodeInvalidInput, fmt.Sprintf("ref %q must not look like an entity ID", ref), nil)
	}
	if _, exists := declared[ref]; exists {
		return newError(CodeInvalidInput, fmt.Sprintf("ref %q is declared more than once", ref), nil)
	}
	return nil
}

func resolveEndpoint(endpoint, role string, refToID map[string]string) (string, error) {
	if endpoint == "" {
		return "", newError(CodeInvalidInput, fmt.Sprintf("%s is required", role), nil)
	}
	if id, declared := refToID[endpoint]; declared {
		return id, nil
	}
	if _, _, _, looksLikeEntityID := model.ParseEntityID(endpoint); looksLikeEntityID {
		return endpoint, nil
	}
	return "", newError(
		CodeInvalidInput,
		fmt.Sprintf("%s %q is neither a ref declared in this batch nor an entity ID", role, endpoint),
		nil,
	)
}

func batchError(subject string, err error) error {
	var engineErr *Error
	if !errors.As(err, &engineErr) {
		return newError(CodeRuntime, fmt.Sprintf("%s: %v", subject, err), err)
	}
	return newError(engineErr.Code, fmt.Sprintf("%s: %s", subject, engineErr.Message), err)
}

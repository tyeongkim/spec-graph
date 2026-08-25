package specgraph

import (
	"context"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

// BootstrapCandidate represents an entity candidate for import.
type BootstrapCandidate struct {
	ID         string
	Type       string
	Title      string
	Confidence float64
}

// BootstrapRelationCandidate represents a relation candidate for import.
type BootstrapRelationCandidate struct {
	From       string
	To         string
	Type       string
	Confidence float64
}

// BootstrapImportRequest describes the inputs for importing bootstrap candidates.
type BootstrapImportRequest struct {
	Entities  []BootstrapCandidate
	Relations []BootstrapRelationCandidate
}

// BootstrapImportResult holds the outcome of a bootstrap import.
type BootstrapImportResult struct {
	Created []string
	Skipped []BootstrapSkippedItem
}

// BootstrapSkippedItem records a candidate that was skipped and why.
type BootstrapSkippedItem struct {
	ID     string
	Reason string
}

// BootstrapImport imports entity and relation candidates as one unit. Malformed
// input aborts the import and leaves the graph untouched, so a candidate is
// validated before its confidence is weighed. Skipped candidates are recorded
// and do not stop the import: confidence below the threshold reflects the
// purpose of candidate filtering, an already-existing entity or relation keeps
// re-importing the same file idempotent, and endpoint types that violate the
// edge matrix are a heuristic extraction artifact.
func (e *Engine) BootstrapImport(ctx context.Context, req BootstrapImportRequest) (BootstrapImportResult, error) {
	return writeLocked(ctx, e, func() (BootstrapImportResult, error) {
		return transact(e, func(tx *txn) (BootstrapImportResult, error) {
			return tx.bootstrapImport(req)
		})
	})
}

func (t *txn) bootstrapImport(req BootstrapImportRequest) (BootstrapImportResult, error) {
	var result BootstrapImportResult

	for _, c := range req.Entities {
		et := model.EntityType(c.Type)
		if _, known := model.TypePrefixMap[et]; !known {
			return BootstrapImportResult{}, newError(CodeInvalidInput, fmt.Sprintf("candidate %q has unknown entity type %q", c.ID, c.Type), nil)
		}
		if err := model.ValidateEntityID(c.ID, et); err != nil {
			return BootstrapImportResult{}, newError(CodeInvalidInput, err.Error(), err)
		}

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: c.ID, Reason: "low confidence",
			})
			continue
		}

		if t.exists(c.ID, et) {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: c.ID, Reason: "already exists",
			})
			continue
		}

		if err := t.write(&spectoml.EntityFile{
			Schema: 1,
			ID:     c.ID,
			Type:   et,
			Title:  c.Title,
			Status: model.EntityStatusDraft,
		}); err != nil {
			return BootstrapImportResult{}, err
		}

		result.Created = append(result.Created, c.ID)
	}

	entities := &stagedEntityFetcher{tx: t}

	for _, c := range req.Relations {
		key := fmt.Sprintf("%s:%s:%s", c.From, c.To, c.Type)

		rt := model.RelationType(c.Type)
		if !isValidRelationType(rt) {
			return BootstrapImportResult{}, newError(CodeInvalidInput, fmt.Sprintf("candidate %q has unknown relation type %q", key, c.Type), nil)
		}

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "low confidence",
			})
			continue
		}

		from, err := entities.Get(c.From)
		if err != nil {
			return BootstrapImportResult{}, lookupError(fmt.Sprintf("lookup from entity %q of candidate %q", c.From, key), c.From, err)
		}
		to, err := entities.Get(c.To)
		if err != nil {
			return BootstrapImportResult{}, lookupError(fmt.Sprintf("lookup to entity %q of candidate %q", c.To, key), c.To, err)
		}

		if !model.IsEdgeAllowed(rt, from.Type, to.Type, nil) {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "invalid edge",
			})
			continue
		}

		ownerID := c.From
		ownerType := from.Type
		targetID := c.To
		if spectoml.IsSymmetricRelation(rt) && c.From > c.To {
			ownerID = c.To
			ownerType = to.Type
			targetID = c.From
		}

		owner, err := t.read(ownerID, ownerType)
		if err != nil {
			return BootstrapImportResult{}, err
		}

		if hasRelation(owner, targetID, rt) {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "already exists",
			})
			continue
		}

		owner.Relations = append(owner.Relations, spectoml.RelationEntry{
			To:   targetID,
			Type: rt,
		})

		if err := t.write(owner); err != nil {
			return BootstrapImportResult{}, err
		}

		result.Created = append(result.Created, key)
	}

	return result, nil
}

func hasRelation(ef *spectoml.EntityFile, targetID string, rt model.RelationType) bool {
	for _, existing := range ef.Relations {
		if existing.To == targetID && existing.Type == rt {
			return true
		}
	}
	return false
}

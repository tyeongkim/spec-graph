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
	Errors  []BootstrapErrorItem
}

// BootstrapSkippedItem records a candidate that was skipped and why.
type BootstrapSkippedItem struct {
	ID     string
	Reason string
}

// BootstrapErrorItem records a candidate that failed to import and the error.
type BootstrapErrorItem struct {
	ID    string
	Error string
}

// BootstrapImport imports entity and relation candidates into the graph.
// Candidates below a confidence threshold are skipped, as are entities that
// already exist. Relation endpoints resolve against entities created earlier in
// the same import; relations with missing endpoints, disallowed edges, or
// existing duplicates are skipped or reported.
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
		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: c.ID, Reason: "low confidence",
			})
			continue
		}

		et := model.EntityType(c.Type)
		if _, known := model.TypePrefixMap[et]; !known {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: c.ID, Error: fmt.Sprintf("unknown entity type %q", c.Type),
			})
			continue
		}
		if err := model.ValidateEntityID(c.ID, et); err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: c.ID, Error: err.Error(),
			})
			continue
		}

		if t.exists(c.ID, et) {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: c.ID, Reason: "already exists",
			})
			continue
		}

		ef := &spectoml.EntityFile{
			Schema: 1,
			ID:     c.ID,
			Type:   et,
			Title:  c.Title,
			Status: model.EntityStatusDraft,
		}

		if err := t.write(ef); err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: c.ID, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, c.ID)
	}

	entities := &stagedEntityFetcher{tx: t}

	for _, c := range req.Relations {
		key := fmt.Sprintf("%s:%s:%s", c.From, c.To, c.Type)

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "low confidence",
			})
			continue
		}

		rt := model.RelationType(c.Type)

		from, err := entities.Get(c.From)
		if err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("from entity %q not found", c.From),
			})
			continue
		}
		to, err := entities.Get(c.To)
		if err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("to entity %q not found", c.To),
			})
			continue
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
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("read owner entity: %v", err),
			})
			continue
		}

		duplicate := false
		for _, existing := range owner.Relations {
			if existing.To == targetID && existing.Type == rt {
				duplicate = true
				break
			}
		}
		if duplicate {
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
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, key)
	}

	return result, nil
}

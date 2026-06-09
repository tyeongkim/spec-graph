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

// BootstrapImport imports entity and relation candidates into the graph via the
// TOML store. Candidates below a confidence threshold are skipped, as are
// entities that already exist. After writing entities the index is rebuilt so
// relation endpoints can be validated against it; relations with missing
// endpoints, disallowed edges, or existing duplicates are skipped or reported.
// The provided context is accepted for forward compatibility and is not yet
// observed.
func (e *Engine) BootstrapImport(ctx context.Context, req BootstrapImportRequest) (BootstrapImportResult, error) {
	_ = ctx

	e.mu.Lock()
	defer e.mu.Unlock()

	var result BootstrapImportResult

	for _, c := range req.Entities {
		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: c.ID, Reason: "low confidence",
			})
			continue
		}

		et := model.EntityType(c.Type)
		if e.store.EntityExists(c.ID, et) {
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

		if err := e.store.WriteEntity(ef); err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: c.ID, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, c.ID)
	}

	if err := e.syncer.ForceRebuild(); err != nil {
		result.Errors = append(result.Errors, BootstrapErrorItem{
			ID: "_rebuild", Error: fmt.Sprintf("index rebuild after entities: %s", err.Error()),
		})
		return result, nil
	}

	for _, c := range req.Relations {
		key := fmt.Sprintf("%s:%s:%s", c.From, c.To, c.Type)

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "low confidence",
			})
			continue
		}

		rt := model.RelationType(c.Type)

		fromRec, err := e.idx.GetEntity(c.From)
		if err != nil || fromRec == nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("from entity %q not found", c.From),
			})
			continue
		}
		toRec, err := e.idx.GetEntity(c.To)
		if err != nil || toRec == nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("to entity %q not found", c.To),
			})
			continue
		}

		fromType := model.EntityType(fromRec.Type)
		toType := model.EntityType(toRec.Type)
		if !model.IsEdgeAllowed(rt, fromType, toType, nil) {
			result.Skipped = append(result.Skipped, BootstrapSkippedItem{
				ID: key, Reason: "invalid edge",
			})
			continue
		}

		ownerID := c.From
		ownerType := fromType
		targetID := c.To
		if isSymmetricRelation(rt) && c.From > c.To {
			ownerID = c.To
			ownerType = toType
			targetID = c.From
		}

		ownerEF, err := e.store.ReadEntity(ownerID, ownerType)
		if err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: fmt.Sprintf("read owner entity: %v", err),
			})
			continue
		}

		duplicate := false
		for _, existing := range ownerEF.Relations {
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

		ownerEF.Relations = append(ownerEF.Relations, spectoml.RelationEntry{
			To:   targetID,
			Type: rt,
		})

		if err := e.store.WriteEntity(ownerEF); err != nil {
			result.Errors = append(result.Errors, BootstrapErrorItem{
				ID: key, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, key)
	}

	return result, nil
}

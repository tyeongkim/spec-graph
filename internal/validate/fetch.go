package validate

import (
	"errors"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// A check reaching a fetcher has two distinct failure modes that the original
// `if err != nil { continue }` conflated:
//
//   - The entity does not exist. Checks legitimately skip these; a dangling
//     reference is either not this check's concern or reported by another check.
//   - Storage failed. Skipping here lets a broken index report as a clean graph,
//     so the check must surface the failure as an issue instead.
//
// fetchEntity and fetchRelations separate the two. Callers skip on (nil, nil)
// and record the returned issue when one is present.

// fetchEntity retrieves an entity for a check. It returns a nil entity and nil
// issue when the entity simply does not exist, and a high-severity issue when
// the fetch itself failed.
func fetchEntity(ef EntityFetcher, id, check string, layer model.Layer) (*model.Entity, *ValidationIssue) {
	e, err := ef.Get(id)
	if err == nil {
		return &e, nil
	}
	if isNotFound(err) {
		return nil, nil
	}
	return nil, &ValidationIssue{
		Check:    check,
		Severity: SeverityHigh,
		Entity:   id,
		Message:  fmt.Sprintf("could not read entity %s: %v", id, err),
		Layer:    layer,
	}
}

// fetchRelations retrieves an entity's relations for a check. A fetch failure
// yields a high-severity issue rather than being skipped.
func fetchRelations(rf RelationFetcher, id, check string, layer model.Layer) ([]model.Relation, *ValidationIssue) {
	rels, err := rf.GetByEntity(id)
	if err == nil {
		return rels, nil
	}
	if isNotFound(err) {
		return nil, nil
	}
	return nil, &ValidationIssue{
		Check:    check,
		Severity: SeverityHigh,
		Entity:   id,
		Message:  fmt.Sprintf("could not read relations for %s: %v", id, err),
		Layer:    layer,
	}
}

// listFailureIssue reports a failed entity listing. A check that cannot read its
// own subject set must say so rather than returning no issues.
func listFailureIssue(check string, layer model.Layer, err error) ValidationIssue {
	return ValidationIssue{
		Check:    check,
		Severity: SeverityHigh,
		Entity:   "",
		Message:  fmt.Sprintf("could not list %s entities: %v", layer, err),
		Layer:    layer,
	}
}

func isNotFound(err error) bool {
	var entityNotFound *model.ErrEntityNotFound
	var relationNotFound *model.ErrRelationNotFound
	return errors.As(err, &entityNotFound) || errors.As(err, &relationNotFound)
}

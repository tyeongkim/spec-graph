package validate

import (
	"errors"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func fetchEntity(ef EntityFetcher, id, check string, layer model.Layer) (*model.Entity, *ValidationIssue) {
	e, err := ef.Get(id)
	if err == nil {
		return &e, nil
	}
	if isNotFound(err) {
		return nil, nil
	}
	return nil, &ValidationIssue{
		Check:            check,
		Severity:         SeverityHigh,
		Entity:           id,
		Message:          fmt.Sprintf("could not read entity %s: %v", id, err),
		Layer:            layer,
		executionFailure: true,
	}
}

func fetchRelations(rf RelationFetcher, id, check string, layer model.Layer) ([]model.Relation, *ValidationIssue) {
	rels, err := rf.GetByEntity(id)
	if err == nil {
		return rels, nil
	}
	if isNotFound(err) {
		return nil, nil
	}
	return nil, &ValidationIssue{
		Check:            check,
		Severity:         SeverityHigh,
		Entity:           id,
		Message:          fmt.Sprintf("could not read relations for %s: %v", id, err),
		Layer:            layer,
		executionFailure: true,
	}
}

func listFailureIssue(check string, layer model.Layer, err error) ValidationIssue {
	return ValidationIssue{
		Check:            check,
		Severity:         SeverityHigh,
		Entity:           "",
		Message:          fmt.Sprintf("could not list %s entities: %v", layer, err),
		Layer:            layer,
		executionFailure: true,
	}
}

func subjectResolutionIssue(check string, layer model.Layer, err error) ValidationIssue {
	return ValidationIssue{
		Check:            check,
		Severity:         SeverityHigh,
		Message:          fmt.Sprintf("could not resolve validation subjects: %v", err),
		Layer:            layer,
		executionFailure: true,
	}
}

func isNotFound(err error) bool {
	var entityNotFound *model.ErrEntityNotFound
	var relationNotFound *model.ErrRelationNotFound
	return errors.As(err, &entityNotFound) || errors.As(err, &relationNotFound)
}

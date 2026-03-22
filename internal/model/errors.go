package model

import "fmt"

type ErrEntityNotFound struct {
	ID string
}

func (e *ErrEntityNotFound) Error() string {
	return fmt.Sprintf("entity %q not found", e.ID)
}

type ErrRelationNotFound struct {
	ID int
}

func (e *ErrRelationNotFound) Error() string {
	return fmt.Sprintf("relation %d not found", e.ID)
}

type ErrInvalidEdge struct {
	FromType     EntityType
	ToType       EntityType
	RelationType RelationType
}

func (e *ErrInvalidEdge) Error() string {
	return fmt.Sprintf("relation %q not allowed from %q to %q", e.RelationType, e.FromType, e.ToType)
}

type ErrDuplicateEntity struct {
	ID string
}

func (e *ErrDuplicateEntity) Error() string {
	return fmt.Sprintf("entity %q already exists", e.ID)
}

type ErrDuplicateRelation struct {
	FromID       string
	ToID         string
	RelationType RelationType
}

func (e *ErrDuplicateRelation) Error() string {
	return fmt.Sprintf("relation %q from %q to %q already exists", e.RelationType, e.FromID, e.ToID)
}

type ErrSelfLoop struct {
	ID string
}

func (e *ErrSelfLoop) Error() string {
	return fmt.Sprintf("self-loop not allowed on entity %q", e.ID)
}

type ErrInvalidInput struct {
	Message string
}

func (e *ErrInvalidInput) Error() string {
	return fmt.Sprintf("invalid input: %s", e.Message)
}

type ErrChangesetNotFound struct {
	ID string
}

func (e *ErrChangesetNotFound) Error() string {
	return fmt.Sprintf("changeset %q not found", e.ID)
}

type ErrNotInitialized struct{}

func (e *ErrNotInitialized) Error() string {
	return "database not initialized; run 'spec-graph init' first"
}

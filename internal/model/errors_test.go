package model

import (
	"errors"
	"strings"
	"testing"
)

func TestErrEntityNotFound(t *testing.T) {
	err := &ErrEntityNotFound{ID: "REQ-001"}
	var target *ErrEntityNotFound

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrEntityNotFound")
	}
	if !strings.Contains(err.Error(), "REQ-001") {
		t.Errorf("error message %q should contain entity ID", err.Error())
	}
}

func TestErrRelationNotFound(t *testing.T) {
	err := &ErrRelationNotFound{ID: 42}
	var target *ErrRelationNotFound

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrRelationNotFound")
	}
	if !strings.Contains(err.Error(), "42") {
		t.Errorf("error message %q should contain relation ID", err.Error())
	}
}

func TestErrInvalidEdge(t *testing.T) {
	err := &ErrInvalidEdge{
		FromType:     EntityTypeTest,
		ToType:       EntityTypePhase,
		RelationType: RelationImplements,
	}
	var target *ErrInvalidEdge

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrInvalidEdge")
	}
	msg := err.Error()
	if !strings.Contains(msg, "test") || !strings.Contains(msg, "phase") || !strings.Contains(msg, "implements") {
		t.Errorf("error message %q should contain from type, to type, and relation type", msg)
	}
}

func TestErrDuplicateEntity(t *testing.T) {
	err := &ErrDuplicateEntity{ID: "REQ-001"}
	var target *ErrDuplicateEntity

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrDuplicateEntity")
	}
	if !strings.Contains(err.Error(), "REQ-001") {
		t.Errorf("error message %q should contain entity ID", err.Error())
	}
}

func TestErrDuplicateRelation(t *testing.T) {
	err := &ErrDuplicateRelation{
		FromID:       "REQ-001",
		ToID:         "DEC-001",
		RelationType: RelationDependsOn,
	}
	var target *ErrDuplicateRelation

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrDuplicateRelation")
	}
	msg := err.Error()
	if !strings.Contains(msg, "REQ-001") || !strings.Contains(msg, "DEC-001") {
		t.Errorf("error message %q should contain from and to IDs", msg)
	}
}

func TestErrSelfLoop(t *testing.T) {
	err := &ErrSelfLoop{ID: "REQ-001"}
	var target *ErrSelfLoop

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrSelfLoop")
	}
	if !strings.Contains(err.Error(), "REQ-001") {
		t.Errorf("error message %q should contain entity ID", err.Error())
	}
}

func TestErrInvalidInput(t *testing.T) {
	err := &ErrInvalidInput{Message: "bad input"}
	var target *ErrInvalidInput

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrInvalidInput")
	}
	if !strings.Contains(err.Error(), "bad input") {
		t.Errorf("error message %q should contain the message", err.Error())
	}
}

func TestErrNotInitialized(t *testing.T) {
	err := &ErrNotInitialized{}
	var target *ErrNotInitialized

	if !errors.As(err, &target) {
		t.Error("expected errors.As to match ErrNotInitialized")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestAllErrorsImplementErrorInterface(t *testing.T) {
	errs := []error{
		&ErrEntityNotFound{ID: "X"},
		&ErrRelationNotFound{ID: 1},
		&ErrInvalidEdge{FromType: EntityTypeTest, ToType: EntityTypePhase, RelationType: RelationImplements},
		&ErrDuplicateEntity{ID: "X"},
		&ErrDuplicateRelation{FromID: "A", ToID: "B", RelationType: RelationDependsOn},
		&ErrSelfLoop{ID: "X"},
		&ErrInvalidInput{Message: "msg"},
		&ErrNotInitialized{},
	}

	for _, err := range errs {
		if err.Error() == "" {
			t.Errorf("%T.Error() returned empty string", err)
		}
	}
}

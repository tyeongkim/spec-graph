package model

import "fmt"

// IsExecType reports whether t is a plan, phase, task, or change.
func IsExecType(t EntityType) bool {
	return LayerForEntityType(t) == LayerExec
}

// ExecLifecycleStarted reports whether an exec entity has left draft, so its
// scope, contract, and ordering are no longer free to change.
func ExecLifecycleStarted(status EntityStatus) bool {
	switch status {
	case EntityStatusActive, EntityStatusResolved, EntityStatusDeprecated, EntityStatusDeleted:
		return true
	}
	return false
}

// ValidateExecTransition rejects lifecycle transitions outside the strict exec
// state machine: draft → active|deprecated, active → resolved|deprecated, and no
// exit from resolved, deprecated, or deleted.
func ValidateExecTransition(entityType EntityType, from, to EntityStatus) error {
	valid := (from == EntityStatusDraft && (to == EntityStatusActive || to == EntityStatusDeprecated)) ||
		(from == EntityStatusActive && (to == EntityStatusResolved || to == EntityStatusDeprecated))
	if !valid {
		return fmt.Errorf("%s status transition %q to %q is not allowed", entityType, from, to)
	}
	return nil
}

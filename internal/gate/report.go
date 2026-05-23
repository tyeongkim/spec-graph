package gate

import (
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// Report is the outcome of a gate enforcement check. If Blocked is true,
// the status transition should not proceed.
type Report struct {
	// Blocked indicates whether the transition is denied.
	Blocked bool `json:"blocked"`
	// BlockingIssues lists validation issues that prevent the transition.
	BlockingIssues []validate.ValidationIssue `json:"blocking_issues,omitempty"`
	// Warnings lists low-severity issues that do not block the transition.
	Warnings []validate.ValidationIssue `json:"warnings,omitempty"`
	// EntityID is the entity being transitioned.
	EntityID string `json:"entity_id"`
	// EntityType is the type of the entity being transitioned.
	EntityType model.EntityType `json:"entity_type"`
	// FromStatus is the current status before the transition.
	FromStatus model.EntityStatus `json:"from_status"`
	// ToStatus is the requested target status.
	ToStatus model.EntityStatus `json:"to_status"`
}

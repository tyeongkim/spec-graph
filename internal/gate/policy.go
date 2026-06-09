package gate

import (
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// Policy defines which validation checks to enforce for a gated transition
// and which severities block the transition.
type Policy struct {
	Checks             []string
	BlockingSeverities []validate.Severity
}

var defaultBlockingSeverities = []validate.Severity{
	validate.SeverityHigh,
	validate.SeverityMedium,
}

var phasePolicies = map[transitionKey]Policy{
	{toStatus: "resolved"}: {
		Checks:             []string{"delivery_completeness", "gates"},
		BlockingSeverities: defaultBlockingSeverities,
	},
}

var planPolicies = map[transitionKey]Policy{
	{toStatus: "resolved"}: {
		Checks:             []string{"plan_coverage"},
		BlockingSeverities: defaultBlockingSeverities,
	},
}

type transitionKey struct {
	toStatus string
}

// LookupPolicy returns the gate policy for the given target, or nil if no
// gate applies.
func LookupPolicy(t Target) *Policy {
	key := transitionKey{toStatus: string(t.ToStatus)}

	var policies map[transitionKey]Policy
	switch t.EntityType {
	case model.EntityTypePhase:
		policies = phasePolicies
	case model.EntityTypePlan:
		policies = planPolicies
	default:
		return nil
	}

	p, ok := policies[key]
	if !ok {
		return nil
	}
	return &p
}

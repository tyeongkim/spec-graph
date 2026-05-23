// Package gate enforces validation gates on entity status transitions.
// It orchestrates calls to the validate package; it does not contain
// validation logic itself.
package gate

import (
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// Target describes the status transition being attempted.
type Target struct {
	EntityID   string
	EntityType model.EntityType
	FromStatus model.EntityStatus
	ToStatus   model.EntityStatus
}

// Enforce checks whether a status transition is allowed by running the
// applicable validation gate. Returns a Report; if Report.Blocked is true,
// the transition should not proceed.
func Enforce(target Target, rf validate.RelationFetcher, ef validate.EntityFetcher) (*Report, error) {
	policy := LookupPolicy(target)
	if policy == nil {
		return &Report{
			EntityID:   target.EntityID,
			EntityType: target.EntityType,
			FromStatus: target.FromStatus,
			ToStatus:   target.ToStatus,
		}, nil
	}

	opts := buildValidateOptions(target, policy)

	result, err := validate.Validate(opts, rf, ef)
	if err != nil {
		return nil, fmt.Errorf("gate enforce %s: %w", target.EntityID, err)
	}

	return buildReport(target, result, policy), nil
}

func buildValidateOptions(target Target, policy *Policy) validate.ValidateOptions {
	layer := model.LayerMapping

	opts := validate.ValidateOptions{
		Checks: policy.Checks,
		Layer:  &layer,
	}

	if target.EntityType == model.EntityTypePhase {
		opts.Phase = &target.EntityID
	}

	return opts
}

func buildReport(target Target, result *validate.ValidateResult, policy *Policy) *Report {
	blocking := severitySet(policy.BlockingSeverities)

	report := &Report{
		EntityID:   target.EntityID,
		EntityType: target.EntityType,
		FromStatus: target.FromStatus,
		ToStatus:   target.ToStatus,
	}

	for _, issue := range result.Issues {
		if blocking[issue.Severity] {
			report.BlockingIssues = append(report.BlockingIssues, issue)
		} else {
			report.Warnings = append(report.Warnings, issue)
		}
	}

	report.Blocked = len(report.BlockingIssues) > 0
	return report
}

func severitySet(sevs []validate.Severity) map[validate.Severity]bool {
	m := make(map[validate.Severity]bool, len(sevs))
	for _, s := range sevs {
		m[s] = true
	}
	return m
}

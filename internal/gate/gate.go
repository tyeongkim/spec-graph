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
	Candidate  model.Entity
	RepoRoot   string
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

	overlay := newOverlayEntityFetcher(ef, target.Candidate)
	structuralResult, err := runChecks(target, policy.StructuralChecks, rf, overlay)
	if err != nil {
		return nil, fmt.Errorf("gate enforce %s: %w", target.EntityID, err)
	}
	completionResult, err := runChecks(target, policy.Checks, rf, overlay)
	if err != nil {
		return nil, fmt.Errorf("gate enforce %s: %w", target.EntityID, err)
	}

	structuralIssues, completionIssues := evaluateLifecycle(target, rf, overlay)
	structuralResult.Issues = append(structuralResult.Issues, structuralIssues...)
	completionResult.Issues = append(completionResult.Issues, completionIssues...)

	return buildReport(target, structuralResult, completionResult, policy), nil
}

func runChecks(target Target, checks []string, rf validate.RelationFetcher, ef validate.EntityFetcher) (*validate.ValidateResult, error) {
	opts := validate.ValidateOptions{
		Checks: checks,
	}
	if target.EntityType == model.EntityTypePhase {
		opts.Phase = &target.EntityID
	}
	if target.EntityType == model.EntityTypeTask {
		opts.EntityID = target.EntityID
	}
	if target.EntityType == model.EntityTypePlan {
		opts.Plan = &target.EntityID
	}
	if len(checks) == 0 {
		return &validate.ValidateResult{}, nil
	}
	return validate.Validate(opts, rf, ef)
}

func buildReport(target Target, structural, completion *validate.ValidateResult, policy *Policy) *Report {
	blocking := severitySet(policy.BlockingSeverities)

	report := &Report{
		EntityID:   target.EntityID,
		EntityType: target.EntityType,
		FromStatus: target.FromStatus,
		ToStatus:   target.ToStatus,
	}

	for _, issue := range structural.Issues {
		if blocking[issue.Severity] {
			report.BlockingIssues = append(report.BlockingIssues, issue)
			report.StructuralBlocked = true
		} else {
			report.Warnings = append(report.Warnings, issue)
		}
	}
	for _, issue := range completion.Issues {
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

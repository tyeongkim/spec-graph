package validate

import (
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// Validate runs layered validation checks and returns combined results.
// It dispatches to per-layer validators based on opts.Layer, aggregates
// issues, and optionally filters by EntityID or Phase.
func Validate(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) (*ValidateResult, error) {
	if err := validateCheckNames(opts); err != nil {
		return nil, err
	}

	var allIssues []ValidationIssue
	var satisfactionReports []PhaseSatisfaction

	runArch := opts.Layer == nil || *opts.Layer == model.LayerArch
	runExec := opts.Layer == nil || *opts.Layer == model.LayerExec
	runMapping := opts.Layer == nil || *opts.Layer == model.LayerMapping

	if runArch {
		issues := validateArch(opts, rf, ef)
		allIssues = append(allIssues, issues...)
	}
	if runExec {
		issues := validateExec(opts, rf, ef)
		allIssues = append(allIssues, issues...)
	}
	if runMapping {
		issues, reports := validateMapping(opts, rf, ef)
		allIssues = append(allIssues, issues...)
		satisfactionReports = append(satisfactionReports, reports...)
	}

	if opts.EntityID != "" {
		filtered := allIssues[:0]
		for _, issue := range allIssues {
			if issue.Entity == opts.EntityID {
				filtered = append(filtered, issue)
			}
		}
		allIssues = filtered
	}

	if opts.Phase != nil {
		phaseScope, err := phaseEntityScope(*opts.Phase, rf, ef)
		if err == nil {
			filtered := allIssues[:0]
			for _, issue := range allIssues {
				if issue.Check == "phase_satisfaction" || phaseScope[issue.Entity] {
					filtered = append(filtered, issue)
				}
			}
			allIssues = filtered
		}
	}

	bySeverity := make(map[Severity]int)
	for _, issue := range allIssues {
		bySeverity[issue.Severity]++
	}

	return &ValidateResult{
		Valid:  len(allIssues) == 0,
		Issues: allIssues,
		Summary: ValidateSummary{
			TotalIssues: len(allIssues),
			BySeverity:  bySeverity,
		},
		Satisfaction: satisfactionReports,
	}, nil
}

func phaseEntityScope(phaseID string, rf RelationFetcher, ef EntityFetcher) (map[string]bool, error) {
	effective, err := graph.EffectivePhaseScope(phaseID, rf)
	if err != nil {
		return nil, err
	}

	scope := map[string]bool{phaseID: true}
	for _, id := range effective.Covered {
		scope[id] = true
	}
	for _, id := range effective.Delivered {
		scope[id] = true
	}
	subjects, err := resolveValidationSubjects(ValidateOptions{Phase: &phaseID}, rf, ef)
	if err != nil {
		return nil, err
	}
	for id := range subjects.execIDs {
		scope[id] = true
	}
	return scope, nil
}

func validateCheckNames(opts ValidateOptions) error {
	for _, c := range opts.Checks {
		layer, known := CheckLayer(c)
		if !known {
			return fmt.Errorf("unknown check: %q", c)
		}
		if opts.Layer != nil && layer != *opts.Layer {
			return fmt.Errorf("check %q belongs to layer %q, not %q", c, layer, *opts.Layer)
		}
	}
	return nil
}

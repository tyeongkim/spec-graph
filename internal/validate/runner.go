package validate

import (
	"fmt"

	"github.com/taeyeong/spec-graph/internal/model"
)

// Validate runs layered validation checks and returns combined results.
// It dispatches to per-layer validators based on opts.Layer, aggregates
// issues, and optionally filters by EntityID or Phase.
func Validate(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) (*ValidateResult, error) {
	if err := validateCheckNames(opts); err != nil {
		return nil, err
	}

	var allIssues []ValidationIssue

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
		issues := validateMapping(opts, rf, ef)
		allIssues = append(allIssues, issues...)
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
		phaseScope, err := phaseEntityScope(*opts.Phase, rf)
		if err == nil && len(phaseScope) > 0 {
			filtered := allIssues[:0]
			for _, issue := range allIssues {
				if phaseScope[issue.Entity] || issue.Entity == *opts.Phase {
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
	}, nil
}

func phaseEntityScope(phaseID string, rf RelationFetcher) (map[string]bool, error) {
	rels, err := rf.GetByEntity(phaseID)
	if err != nil {
		return nil, err
	}

	scope := make(map[string]bool)
	for _, r := range rels {
		if r.FromID == phaseID && (r.Type == model.RelationCovers || r.Type == model.RelationDelivers) {
			scope[r.ToID] = true
		}
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

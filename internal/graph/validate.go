package graph

import (
	"fmt"

	"github.com/taeyeong/spec-graph/internal/model"
)

// Validate runs the specified validation checks and returns combined results.
// If opts.Checks is nil or empty, all checks are run.
func Validate(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) (*ValidateResult, error) {
	checks := opts.Checks
	if len(checks) == 0 {
		checks = []string{"orphans", "coverage", "invalid_edges", "superseded_refs"}
	}

	var allIssues []ValidationIssue

	for _, check := range checks {
		var issues []ValidationIssue
		var err error
		switch check {
		case "orphans":
			issues, err = checkOrphans(ef, rf)
		case "coverage":
			issues, err = checkCoverage(ef, rf)
		case "invalid_edges":
			issues, err = checkInvalidEdges(ef, rf)
		case "superseded_refs":
			issues, err = checkSupersededRefs(ef, rf)
		default:
			return nil, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown check: %q", check)}
		}
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, issues...)
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

// checkCoverage verifies that active entities have required coverage relations.
// Four sub-checks:
// 1. Active requirement must have at least one "implements" relation (as to_id)
// 2. Active requirement must have at least one "has_criterion" relation (as from_id)
// 3. Active criterion must have at least one "verifies" relation (as to_id)
// 4. Interface with "triggers" relation must have a "verifies" relation from a test (as to_id)
func checkCoverage(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}

		switch e.Type {
		case model.EntityTypeRequirement:
			hasImplements := false
			for _, r := range rels {
				if r.Type == model.RelationImplements && r.ToID == e.ID {
					hasImplements = true
					break
				}
			}
			if !hasImplements {
				issues = append(issues, ValidationIssue{
					Check:    "coverage",
					Severity: SeverityHigh,
					Entity:   e.ID,
					Message:  "requirement has no implementation",
				})
			}

			hasCriterion := false
			for _, r := range rels {
				if r.Type == model.RelationHasCriterion && r.FromID == e.ID {
					hasCriterion = true
					break
				}
			}
			if !hasCriterion {
				issues = append(issues, ValidationIssue{
					Check:    "coverage",
					Severity: SeverityHigh,
					Entity:   e.ID,
					Message:  "requirement has no acceptance criterion",
				})
			}

		case model.EntityTypeCriterion:
			hasVerifies := false
			for _, r := range rels {
				if r.Type == model.RelationVerifies && r.ToID == e.ID {
					hasVerifies = true
					break
				}
			}
			if !hasVerifies {
				issues = append(issues, ValidationIssue{
					Check:    "coverage",
					Severity: SeverityHigh,
					Entity:   e.ID,
					Message:  "criterion has no verification",
				})
			}

		case model.EntityTypeInterface:
			hasTriggers := false
			for _, r := range rels {
				if r.Type == model.RelationTriggers && r.FromID == e.ID {
					hasTriggers = true
					break
				}
			}
			if hasTriggers {
				hasVerifyingTest := false
				for _, r := range rels {
					if r.Type == model.RelationVerifies && r.ToID == e.ID {
						hasVerifyingTest = true
						break
					}
				}
				if !hasVerifyingTest {
					issues = append(issues, ValidationIssue{
						Check:    "coverage",
						Severity: SeverityHigh,
						Entity:   e.ID,
						Message:  "interface triggers state but has no verifying test",
					})
				}
			}
		}
	}

	return issues, nil
}

// checkInvalidEdges validates all relations against the edge matrix.
// For each relation, fetches source and target entity types and calls
// model.IsEdgeAllowed. Any violation is reported as a high-severity issue.
func checkInvalidEdges(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}

		for _, rel := range rels {
			key := fmt.Sprintf("%s|%s|%s", rel.FromID, rel.ToID, rel.Type)
			if seen[key] {
				continue
			}
			seen[key] = true

			srcEntity, err := ef.Get(rel.FromID)
			if err != nil {
				return nil, fmt.Errorf("get source entity %q: %w", rel.FromID, err)
			}
			tgtEntity, err := ef.Get(rel.ToID)
			if err != nil {
				return nil, fmt.Errorf("get target entity %q: %w", rel.ToID, err)
			}

			if !model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type) {
				issues = append(issues, ValidationIssue{
					Check:    "invalid_edges",
					Severity: SeverityHigh,
					Entity:   rel.FromID,
					Message:  fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type),
				})
			}
		}
	}

	return issues, nil
}

// checkSupersededRefs finds active/draft entities that still reference superseded entities.
func checkSupersededRefs(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	seen := make(map[string]bool)
	var allRels []model.Relation
	for _, e := range entities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}
		for _, r := range rels {
			key := fmt.Sprintf("%d|%s|%s|%s", r.ID, r.FromID, r.ToID, r.Type)
			if !seen[key] {
				seen[key] = true
				allRels = append(allRels, r)
			}
		}
	}

	oldIDs := make(map[string]bool)
	for _, r := range allRels {
		if r.Type == model.RelationSupersedes {
			oldIDs[r.ToID] = true
		}
	}

	if len(oldIDs) == 0 {
		return nil, nil
	}

	var issues []ValidationIssue

	for oldID := range oldIDs {
		rels, err := rf.GetByEntity(oldID)
		if err != nil {
			return nil, fmt.Errorf("get relations for superseded %q: %w", oldID, err)
		}

		for _, r := range rels {
			if r.Type == model.RelationSupersedes {
				continue
			}

			if r.ToID == oldID {
				srcEntity, err := ef.Get(r.FromID)
				if err != nil {
					return nil, fmt.Errorf("get source entity %q: %w", r.FromID, err)
				}
				if srcEntity.Status == model.EntityStatusActive || srcEntity.Status == model.EntityStatusDraft {
					issues = append(issues, ValidationIssue{
						Check:    "superseded_refs",
						Severity: SeverityHigh,
						Entity:   srcEntity.ID,
						Message:  fmt.Sprintf("entity still references superseded entity %s via %s", oldID, r.Type),
					})
				}
			} else if r.FromID == oldID {
				tgtEntity, err := ef.Get(r.ToID)
				if err != nil {
					return nil, fmt.Errorf("get target entity %q: %w", r.ToID, err)
				}
				if tgtEntity.Status == model.EntityStatusActive || tgtEntity.Status == model.EntityStatusDraft {
					issues = append(issues, ValidationIssue{
						Check:    "superseded_refs",
						Severity: SeverityHigh,
						Entity:   tgtEntity.ID,
						Message:  fmt.Sprintf("entity still references superseded entity %s via %s", oldID, r.Type),
					})
				}
			}
		}
	}

	return issues, nil
}

// checkOrphans finds entities with no relations.
// Only checks entities with status "active" or "draft".
func checkOrphans(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	var issues []ValidationIssue
	for _, e := range entities {
		if e.Status != model.EntityStatusActive && e.Status != model.EntityStatusDraft {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}

		if len(rels) == 0 {
			issues = append(issues, ValidationIssue{
				Check:    "orphans",
				Severity: SeverityMedium,
				Entity:   e.ID,
				Message:  "entity has no relations",
			})
		}
	}

	return issues, nil
}

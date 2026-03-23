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
		checks = []string{"orphans", "coverage", "invalid_edges", "superseded_refs", "gates", "cycles", "conflicts"}
	}

	if opts.Phase != nil {
		filteredEF, err := newPhaseFilteredEntityFetcher(ef, rf, *opts.Phase)
		if err != nil {
			return nil, err
		}
		ef = filteredEF
		rf = newPhaseFilteredRelationFetcher(rf, filteredEF.allowed)
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
		case "gates":
			issues, err = checkGates(ef, rf)
		case "cycles":
			issues, err = checkCycles(ef, rf)
		case "conflicts":
			issues, err = checkConflicts(ef, rf)
		default:
			return nil, &model.ErrInvalidInput{Message: fmt.Sprintf("unknown check: %q", check)}
		}
		if err != nil {
			return nil, err
		}
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

func checkGates(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	var issues []ValidationIssue

	plannedIn := make(map[string]string)
	deliveredIn := make(map[string]string)
	seenRels := make(map[string]bool)

	for _, e := range entities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}
		for _, r := range rels {
			key := fmt.Sprintf("%d|%s|%s|%s", r.ID, r.FromID, r.ToID, r.Type)
			if seenRels[key] {
				continue
			}
			seenRels[key] = true
			if r.Type == model.RelationPlannedIn {
				plannedIn[r.FromID] = r.ToID
			}
			if r.Type == model.RelationDeliveredIn {
				deliveredIn[r.FromID] = r.ToID
			}
		}
	}

	deliveredSet := make(map[string]bool)
	for id := range deliveredIn {
		deliveredSet[id] = true
	}

	seenAssumptions := make(map[string]bool)

	for _, e := range entities {
		switch {
		case e.Type == model.EntityTypeQuestion &&
			e.Status != model.EntityStatusResolved &&
			e.Status != model.EntityStatusDeleted:
			issues = append(issues, ValidationIssue{
				Check:    "gates",
				Severity: SeverityHigh,
				Entity:   e.ID,
				Message:  fmt.Sprintf("unresolved question %q blocks phase exit", e.ID),
			})

		case e.Type == model.EntityTypeRisk &&
			e.Status != model.EntityStatusResolved &&
			e.Status != model.EntityStatusDeleted:
			rels, err := rf.GetByEntity(e.ID)
			if err != nil {
				return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
			}
			mitigated := false
			for _, r := range rels {
				if r.Type == model.RelationMitigates && r.ToID == e.ID {
					mitigated = true
					break
				}
			}
			if !mitigated {
				issues = append(issues, ValidationIssue{
					Check:    "gates",
					Severity: SeverityHigh,
					Entity:   e.ID,
					Message:  fmt.Sprintf("unmitigated risk %q blocks phase exit", e.ID),
				})
			}

		case e.Type == model.EntityTypeRequirement && e.Status == model.EntityStatusActive:
			rels, err := rf.GetByEntity(e.ID)
			if err != nil {
				return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
			}
			for _, r := range rels {
				if r.Type == model.RelationDependsOn && r.FromID == e.ID {
					target, err := ef.Get(r.ToID)
					if err != nil {
						continue
					}
					if target.Type == model.EntityTypeDecision && target.Status == model.EntityStatusDraft {
						issues = append(issues, ValidationIssue{
							Check:    "gates",
							Severity: SeverityHigh,
							Entity:   e.ID,
							Message:  fmt.Sprintf("entity depends on draft decision %q", r.ToID),
						})
					}
				}
				if r.Type == model.RelationAssumes && r.FromID == e.ID && !seenAssumptions[r.ToID] {
					seenAssumptions[r.ToID] = true
					assumption, err := ef.Get(r.ToID)
					if err != nil {
						continue
					}
					if assumption.Status != model.EntityStatusResolved && assumption.Status != model.EntityStatusDeleted {
						issues = append(issues, ValidationIssue{
							Check:    "gates",
							Severity: SeverityMedium,
							Entity:   assumption.ID,
							Message:  fmt.Sprintf("unresolved assumption %q blocks phase exit", assumption.ID),
						})
					}
				}
			}
		}
	}

	for entityID, phaseID := range plannedIn {
		_ = phaseID
		entity, err := ef.Get(entityID)
		if err != nil {
			continue
		}

		delivered := false
		switch entity.Type {
		case model.EntityTypeInterface, model.EntityTypeState, model.EntityTypeTest, model.EntityTypeDecision:
			delivered = deliveredSet[entityID]

		case model.EntityTypeRequirement:
			rels, err := rf.GetByEntity(entityID)
			if err != nil {
				return nil, fmt.Errorf("get relations for %q: %w", entityID, err)
			}
			for _, r := range rels {
				if r.Type == model.RelationImplements && r.ToID == entityID && deliveredSet[r.FromID] {
					delivered = true
					break
				}
			}

		case model.EntityTypeQuestion:
			delivered = entity.Status == model.EntityStatusResolved

		case model.EntityTypeRisk:
			rels, err := rf.GetByEntity(entityID)
			if err != nil {
				return nil, fmt.Errorf("get relations for %q: %w", entityID, err)
			}
			for _, r := range rels {
				if r.Type == model.RelationMitigates && r.ToID == entityID && deliveredSet[r.FromID] {
					delivered = true
					break
				}
			}

		default:
			delivered = true
		}

		if !delivered {
			issues = append(issues, ValidationIssue{
				Check:    "gates",
				Severity: SeverityHigh,
				Entity:   entityID,
				Message:  fmt.Sprintf("entity %q is planned but not delivered in phase", entityID),
			})
		}
	}

	return issues, nil
}

// checkCycles detects circular references in depends_on relation chains using DFS.
func checkCycles(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	adj := make(map[string][]string)
	seen := make(map[string]bool)
	for _, e := range entities {
		seen[e.ID] = true
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}
		for _, r := range rels {
			if r.Type == model.RelationDependsOn && r.FromID == e.ID {
				adj[e.ID] = append(adj[e.ID], r.ToID)
			}
		}
	}

	var issues []ValidationIssue
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, next := range adj[node] {
			if !seen[next] {
				continue
			}
			if !visited[next] {
				if dfs(next, path) {
					return true
				}
			} else if recStack[next] {
				// Found cycle — extract the cycle portion from path.
				cycleStart := -1
				for i, id := range path {
					if id == next {
						cycleStart = i
						break
					}
				}
				cycle := path[cycleStart:]
				cycleDesc := fmt.Sprintf("%s → %s", formatCyclePath(cycle), next)
				for _, id := range cycle {
					issues = append(issues, ValidationIssue{
						Check:    "cycles",
						Severity: SeverityHigh,
						Entity:   id,
						Message:  fmt.Sprintf("circular dependency detected: %s", cycleDesc),
					})
				}
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, e := range entities {
		if !visited[e.ID] {
			dfs(e.ID, nil)
		}
	}

	return issues, nil
}

// formatCyclePath joins entity IDs with arrows for display.
func formatCyclePath(ids []string) string {
	result := ""
	for i, id := range ids {
		if i > 0 {
			result += " → "
		}
		result += id
	}
	return result
}

// checkConflicts finds pairs of active entities connected by conflicts_with relations.
// If both sides of a conflicts_with relation are active, it's reported as a high-severity issue.
func checkConflicts(ef EntityFetcher, rf RelationFetcher) ([]ValidationIssue, error) {
	entities, err := ef.List(EntityListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			return nil, fmt.Errorf("get relations for %q: %w", e.ID, err)
		}

		for _, r := range rels {
			if r.Type != model.RelationConflictsWith {
				continue
			}

			// Build canonical key to avoid reporting A↔B twice.
			key := r.FromID + "|" + r.ToID
			if r.ToID < r.FromID {
				key = r.ToID + "|" + r.FromID
			}
			if seen[key] {
				continue
			}
			seen[key] = true

			// Check that the other side is also active.
			otherID := r.ToID
			if otherID == e.ID {
				otherID = r.FromID
			}
			other, err := ef.Get(otherID)
			if err != nil {
				return nil, fmt.Errorf("get entity %q: %w", otherID, err)
			}
			if other.Status != model.EntityStatusActive {
				continue
			}

			issues = append(issues, ValidationIssue{
				Check:    "conflicts",
				Severity: SeverityHigh,
				Entity:   e.ID,
				Message:  fmt.Sprintf("active conflict between %s and %s", r.FromID, r.ToID),
			})
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

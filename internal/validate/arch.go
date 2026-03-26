package validate

import (
	"fmt"
	"slices"

	"github.com/taeyeong/spec-graph/internal/model"
)

func validateArch(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	checks := opts.Checks
	if len(checks) == 0 {
		checks = ArchChecks
	}

	var allIssues []ValidationIssue

	for _, check := range checks {
		layer, known := CheckLayer(check)
		if !known || layer != model.LayerArch {
			continue
		}

		var issues []ValidationIssue
		switch check {
		case "orphans":
			issues = checkOrphans(rf, ef)
		case "coverage":
			issues = checkCoverage(rf, ef)
		case "cycles":
			issues = checkCycles(rf, ef)
		case "conflicts":
			issues = checkConflicts(rf, ef)
		case "invalid_edges":
			issues = checkInvalidEdges(rf, ef)
		case "superseded_refs":
			issues = checkSupersededRefs(rf, ef)
		case "unresolved":
			issues = checkUnresolved(rf, ef)
		}

		allIssues = append(allIssues, issues...)
	}

	return allIssues
}

func archEntities(ef EntityFetcher) ([]model.Entity, error) {
	layer := model.LayerArch
	return ef.List(EntityListFilters{Layer: &layer})
}

func isArchEntity(e model.Entity) bool {
	return model.LayerForEntityType(e.Type) == model.LayerArch
}

func isArchRelation(r model.Relation) bool {
	return model.LayerForRelationType(r.Type) == model.LayerArch
}

func archRels(rels []model.Relation) []model.Relation {
	out := make([]model.Relation, 0, len(rels))
	for _, r := range rels {
		if isArchRelation(r) {
			out = append(out, r)
		}
	}
	return out
}

func checkOrphans(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	var issues []ValidationIssue
	for _, e := range entities {
		if e.Status != model.EntityStatusActive && e.Status != model.EntityStatusDraft {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}

		if len(archRels(rels)) == 0 {
			issues = append(issues, ValidationIssue{
				Check:    "orphans",
				Severity: SeverityMedium,
				Entity:   e.ID,
				Message:  "entity has no relations",
				Layer:    model.LayerArch,
			})
		}
	}

	return issues
}

// checkCoverage verifies that active arch entities have required coverage relations.
// 1. Active requirement must have at least one "implements" relation (as to_id).
// 2. Active requirement must have at least one "has_criterion" relation (as from_id).
// 3. Active criterion must have at least one "verifies" relation (as to_id).
// 4. Interface with "triggers" relation must have a "verifies" relation from a test.
func checkCoverage(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}
		rels = archRels(rels)

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
					Layer:    model.LayerArch,
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
					Layer:    model.LayerArch,
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
					Layer:    model.LayerArch,
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
						Layer:    model.LayerArch,
					})
				}
			}
		}
	}

	return issues
}

func checkCycles(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	archIDs := make(map[string]bool, len(entities))
	adj := make(map[string][]string)
	for _, e := range entities {
		archIDs[e.ID] = true
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationDependsOn && r.FromID == e.ID && isArchRelation(r) {
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
			if !archIDs[next] {
				continue
			}
			if !visited[next] {
				if dfs(next, path) {
					return true
				}
			} else if recStack[next] {
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
						Layer:    model.LayerArch,
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

	return issues
}

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

func checkConflicts(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}

		for _, r := range rels {
			if r.Type != model.RelationConflictsWith || !isArchRelation(r) {
				continue
			}

			key := r.FromID + "|" + r.ToID
			if r.ToID < r.FromID {
				key = r.ToID + "|" + r.FromID
			}
			if seen[key] {
				continue
			}
			seen[key] = true

			otherID := r.ToID
			if otherID == e.ID {
				otherID = r.FromID
			}
			other, err := ef.Get(otherID)
			if err != nil {
				continue
			}
			if other.Status != model.EntityStatusActive || !isArchEntity(other) {
				continue
			}

			issues = append(issues, ValidationIssue{
				Check:    "conflicts",
				Severity: SeverityHigh,
				Entity:   e.ID,
				Message:  fmt.Sprintf("active conflict between %s and %s", r.FromID, r.ToID),
				Layer:    model.LayerArch,
			})
		}
	}

	return issues
}

func checkInvalidEdges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}

		for _, rel := range rels {
			if !isArchRelation(rel) {
				continue
			}

			key := fmt.Sprintf("%s|%s|%s", rel.FromID, rel.ToID, rel.Type)
			if seen[key] {
				continue
			}
			seen[key] = true

			srcEntity, err := ef.Get(rel.FromID)
			if err != nil {
				continue
			}
			tgtEntity, err := ef.Get(rel.ToID)
			if err != nil {
				continue
			}

			if !isArchEntity(srcEntity) || !isArchEntity(tgtEntity) {
				continue
			}

			if !model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, nil) {
				issues = append(issues, ValidationIssue{
					Check:    "invalid_edges",
					Severity: SeverityHigh,
					Entity:   rel.FromID,
					Message:  fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type),
					Layer:    model.LayerArch,
				})
			}
		}
	}

	return issues
}

func checkSupersededRefs(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var allRels []model.Relation
	for _, e := range entities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
			continue
		}
		for _, r := range rels {
			if !isArchRelation(r) {
				continue
			}
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
		return nil
	}

	var issues []ValidationIssue

	for oldID := range oldIDs {
		rels, err := rf.GetByEntity(oldID)
		if err != nil {
			continue
		}

		for _, r := range rels {
			if !isArchRelation(r) {
				continue
			}
			if r.Type == model.RelationSupersedes {
				continue
			}

			if r.ToID == oldID {
				srcEntity, err := ef.Get(r.FromID)
				if err != nil {
					continue
				}
				if !isArchEntity(srcEntity) {
					continue
				}
				if srcEntity.Status == model.EntityStatusActive || srcEntity.Status == model.EntityStatusDraft {
					issues = append(issues, ValidationIssue{
						Check:    "superseded_refs",
						Severity: SeverityHigh,
						Entity:   srcEntity.ID,
						Message:  fmt.Sprintf("entity still references superseded entity %s via %s", oldID, r.Type),
						Layer:    model.LayerArch,
					})
				}
			} else if r.FromID == oldID {
				tgtEntity, err := ef.Get(r.ToID)
				if err != nil {
					continue
				}
				if !isArchEntity(tgtEntity) {
					continue
				}
				if tgtEntity.Status == model.EntityStatusActive || tgtEntity.Status == model.EntityStatusDraft {
					issues = append(issues, ValidationIssue{
						Check:    "superseded_refs",
						Severity: SeverityHigh,
						Entity:   tgtEntity.ID,
						Message:  fmt.Sprintf("entity still references superseded entity %s via %s", oldID, r.Type),
						Layer:    model.LayerArch,
					})
				}
			}
		}
	}

	return issues
}

// checkUnresolved finds arch entities that need resolution:
// - Questions in active/draft without an "answers" relation pointing to them.
// - Assumptions in active/draft (flagged as needing validation).
// - Risks in active/draft without a "mitigates" relation pointing to them.
func checkUnresolved(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return nil
	}

	activeOrDraft := []model.EntityStatus{model.EntityStatusActive, model.EntityStatusDraft}

	var issues []ValidationIssue

	for _, e := range entities {
		if !slices.Contains(activeOrDraft, e.Status) {
			continue
		}

		switch e.Type {
		case model.EntityTypeQuestion:
			rels, err := rf.GetByEntity(e.ID)
			if err != nil {
				continue
			}
			hasAnswer := false
			for _, r := range rels {
				if r.Type == model.RelationAnswers && r.ToID == e.ID {
					hasAnswer = true
					break
				}
			}
			if !hasAnswer {
				issues = append(issues, ValidationIssue{
					Check:    "unresolved",
					Severity: SeverityMedium,
					Entity:   e.ID,
					Message:  "question has no answer",
					Layer:    model.LayerArch,
				})
			}

		case model.EntityTypeAssumption:
			issues = append(issues, ValidationIssue{
				Check:    "unresolved",
				Severity: SeverityMedium,
				Entity:   e.ID,
				Message:  "assumption needs validation",
				Layer:    model.LayerArch,
			})

		case model.EntityTypeRisk:
			rels, err := rf.GetByEntity(e.ID)
			if err != nil {
				continue
			}
			hasMitigation := false
			for _, r := range rels {
				if r.Type == model.RelationMitigates && r.ToID == e.ID {
					hasMitigation = true
					break
				}
			}
			if !hasMitigation {
				issues = append(issues, ValidationIssue{
					Check:    "unresolved",
					Severity: SeverityMedium,
					Entity:   e.ID,
					Message:  "risk has no mitigation",
					Layer:    model.LayerArch,
				})
			}
		}
	}

	return issues
}

package validate

import (
	"fmt"
	"slices"

	"github.com/tyeongkim/spec-graph/internal/model"
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

// archIssue builds an arch-layer issue for the given check.
func archIssue(check string, severity Severity, entity, message string) ValidationIssue {
	return ValidationIssue{
		Check:    check,
		Severity: severity,
		Entity:   entity,
		Message:  message,
		Layer:    model.LayerArch,
	}
}

// archRelationsFor fetches an entity's arch-layer relations for a check.
func archRelationsFor(rf RelationFetcher, id, check string) ([]model.Relation, *ValidationIssue) {
	rels, issue := fetchRelations(rf, id, check, model.LayerArch)
	if issue != nil {
		return nil, issue
	}
	return archRels(rels), nil
}

func checkOrphans(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return []ValidationIssue{listFailureIssue("orphans", model.LayerArch, err)}
	}

	var issues []ValidationIssue
	for _, e := range entities {
		if e.Status != model.EntityStatusActive && e.Status != model.EntityStatusDraft {
			continue
		}

		rels, issue := archRelationsFor(rf, e.ID, "orphans")
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		if len(rels) == 0 {
			issues = append(issues, archIssue("orphans", SeverityMedium, e.ID, "entity has no relations"))
		}
	}

	return issues
}

// checkCoverage verifies that active arch entities have required coverage relations.
func checkCoverage(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return []ValidationIssue{listFailureIssue("coverage", model.LayerArch, err)}
	}

	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, issue := archRelationsFor(rf, e.ID, "coverage")
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		switch e.Type {
		case model.EntityTypeRequirement:
			issues = append(issues, requirementCoverage(e, rels)...)
		case model.EntityTypeCriterion:
			issues = append(issues, criterionCoverage(e, rels)...)
		case model.EntityTypeInterface:
			issues = append(issues, interfaceCoverage(e, rels)...)
		}
	}

	return issues
}

// requirementCoverage requires an active requirement to be implemented and to
// carry at least one acceptance criterion.
func requirementCoverage(e model.Entity, rels []model.Relation) []ValidationIssue {
	var issues []ValidationIssue
	if !hasIncoming(rels, model.RelationImplements, e.ID) {
		issues = append(issues, archIssue("coverage", SeverityHigh, e.ID, "requirement has no implementation"))
	}
	if !hasOutgoing(rels, model.RelationHasCriterion, e.ID) {
		issues = append(issues, archIssue("coverage", SeverityHigh, e.ID, "requirement has no acceptance criterion"))
	}
	return issues
}

// criterionCoverage requires an active criterion to be verified.
func criterionCoverage(e model.Entity, rels []model.Relation) []ValidationIssue {
	if hasIncoming(rels, model.RelationVerifies, e.ID) {
		return nil
	}
	return []ValidationIssue{archIssue("coverage", SeverityHigh, e.ID, "criterion has no verification")}
}

// interfaceCoverage requires a state-triggering interface to be verified.
func interfaceCoverage(e model.Entity, rels []model.Relation) []ValidationIssue {
	if !hasOutgoing(rels, model.RelationTriggers, e.ID) {
		return nil
	}
	if hasIncoming(rels, model.RelationVerifies, e.ID) {
		return nil
	}
	return []ValidationIssue{archIssue("coverage", SeverityHigh, e.ID, "interface triggers state but has no verifying test")}
}

func hasIncoming(rels []model.Relation, relType model.RelationType, id string) bool {
	for _, r := range rels {
		if r.Type == relType && r.ToID == id {
			return true
		}
	}
	return false
}

func hasOutgoing(rels []model.Relation, relType model.RelationType, id string) bool {
	for _, r := range rels {
		if r.Type == relType && r.FromID == id {
			return true
		}
	}
	return false
}

func checkCycles(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return []ValidationIssue{listFailureIssue("cycles", model.LayerArch, err)}
	}

	archIDs, adj, issues := buildDependsOnGraph(entities, rf)
	issues = append(issues, findDependencyCycles(entities, archIDs, adj)...)
	return issues
}

// buildDependsOnGraph collects the arch-layer depends_on adjacency for entities.
// Fetch failures are returned as issues rather than dropping edges silently,
// since a missing edge would hide a cycle.
func buildDependsOnGraph(entities []model.Entity, rf RelationFetcher) (map[string]bool, map[string][]string, []ValidationIssue) {
	archIDs := make(map[string]bool, len(entities))
	adj := make(map[string][]string)
	var issues []ValidationIssue

	for _, e := range entities {
		archIDs[e.ID] = true
		rels, issue := fetchRelations(rf, e.ID, "cycles", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationDependsOn && r.FromID == e.ID && isArchRelation(r) {
				adj[e.ID] = append(adj[e.ID], r.ToID)
			}
		}
	}

	return archIDs, adj, issues
}

// findDependencyCycles reports every entity participating in a depends_on cycle.
func findDependencyCycles(entities []model.Entity, archIDs map[string]bool, adj map[string][]string) []ValidationIssue {
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
				continue
			}
			if !recStack[next] {
				continue
			}

			cycle := path[slices.Index(path, next):]
			cycleDesc := fmt.Sprintf("%s → %s", formatCyclePath(cycle), next)
			for _, id := range cycle {
				issues = append(issues, archIssue("cycles", SeverityHigh, id,
					fmt.Sprintf("circular dependency detected: %s", cycleDesc)))
			}
			return true
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
		return []ValidationIssue{listFailureIssue("conflicts", model.LayerArch, err)}
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive {
			continue
		}

		rels, issue := fetchRelations(rf, e.ID, "conflicts", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		for _, r := range rels {
			if r.Type != model.RelationConflictsWith || !isArchRelation(r) {
				continue
			}

			key := unorderedPairKey(r.FromID, r.ToID)
			if seen[key] {
				continue
			}
			seen[key] = true

			otherID := r.ToID
			if otherID == e.ID {
				otherID = r.FromID
			}
			other, issue := fetchEntity(ef, otherID, "conflicts", model.LayerArch)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if other == nil {
				continue
			}
			if other.Status != model.EntityStatusActive || !isArchEntity(*other) {
				continue
			}

			issues = append(issues, archIssue("conflicts", SeverityHigh, e.ID,
				fmt.Sprintf("active conflict between %s and %s", r.FromID, r.ToID)))
		}
	}

	return issues
}

func unorderedPairKey(a, b string) string {
	if b < a {
		a, b = b, a
	}
	return a + "|" + b
}

func checkInvalidEdges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return []ValidationIssue{listFailureIssue("invalid_edges", model.LayerArch, err)}
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		rels, issue := fetchRelations(rf, e.ID, "invalid_edges", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
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

			srcEntity, issue := fetchEntity(ef, rel.FromID, "invalid_edges", model.LayerArch)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			tgtEntity, issue := fetchEntity(ef, rel.ToID, "invalid_edges", model.LayerArch)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if srcEntity == nil || tgtEntity == nil {
				continue
			}

			if !isArchEntity(*srcEntity) || !isArchEntity(*tgtEntity) {
				continue
			}

			if !model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, nil) {
				issues = append(issues, archIssue("invalid_edges", SeverityHigh, rel.FromID,
					fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type)))
			}
		}
	}

	return issues
}

func checkSupersededRefs(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := archEntities(ef)
	if err != nil {
		return []ValidationIssue{listFailureIssue("superseded_refs", model.LayerArch, err)}
	}

	supersededIDs, issues := collectSupersededIDs(entities, rf)
	if len(supersededIDs) == 0 {
		return issues
	}

	for oldID := range supersededIDs {
		rels, issue := fetchRelations(rf, oldID, "superseded_refs", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		issues = append(issues, referencesToSuperseded(oldID, rels, ef)...)
	}

	return issues
}

// collectSupersededIDs returns the set of entities that something supersedes.
func collectSupersededIDs(entities []model.Entity, rf RelationFetcher) (map[string]bool, []ValidationIssue) {
	seen := make(map[string]bool)
	supersededIDs := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range entities {
		rels, issue := fetchRelations(rf, e.ID, "superseded_refs", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		for _, r := range rels {
			if !isArchRelation(r) {
				continue
			}
			key := fmt.Sprintf("%d|%s|%s|%s", r.ID, r.FromID, r.ToID, r.Type)
			if seen[key] {
				continue
			}
			seen[key] = true
			if r.Type == model.RelationSupersedes {
				supersededIDs[r.ToID] = true
			}
		}
	}

	return supersededIDs, issues
}

// conflicts_with is symmetric and stored under the lexicographically smaller ID,
// so a live conflict can point outward from the superseded entity.
func referencesToSuperseded(oldID string, rels []model.Relation, ef EntityFetcher) []ValidationIssue {
	var issues []ValidationIssue

	for _, r := range rels {
		if !isArchRelation(r) || r.Type == model.RelationSupersedes {
			continue
		}

		var referrerID string
		switch {
		case r.ToID == oldID:
			referrerID = r.FromID
		case r.FromID == oldID && r.Type == model.RelationConflictsWith:
			referrerID = r.ToID
		default:
			continue
		}

		referrer, issue := fetchEntity(ef, referrerID, "superseded_refs", model.LayerArch)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		if referrer == nil || !isArchEntity(*referrer) {
			continue
		}
		if referrer.Status != model.EntityStatusActive && referrer.Status != model.EntityStatusDraft {
			continue
		}

		issues = append(issues, archIssue("superseded_refs", SeverityHigh, referrer.ID,
			fmt.Sprintf("entity still references superseded entity %s via %s", oldID, r.Type)))
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
		return []ValidationIssue{listFailureIssue("unresolved", model.LayerArch, err)}
	}

	var issues []ValidationIssue

	for _, e := range entities {
		if e.Status != model.EntityStatusActive && e.Status != model.EntityStatusDraft {
			continue
		}

		switch e.Type {
		case model.EntityTypeQuestion:
			issues = append(issues, missingIncoming(e, model.RelationAnswers, "question has no answer", rf)...)
		case model.EntityTypeAssumption:
			issues = append(issues, archIssue("unresolved", SeverityMedium, e.ID, "assumption needs validation"))
		case model.EntityTypeRisk:
			issues = append(issues, missingIncoming(e, model.RelationMitigates, "risk has no mitigation", rf)...)
		}
	}

	return issues
}

// missingIncoming reports the given message when no relation of relType points
// at the entity.
func missingIncoming(e model.Entity, relType model.RelationType, message string, rf RelationFetcher) []ValidationIssue {
	rels, issue := fetchRelations(rf, e.ID, "unresolved", model.LayerArch)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	if hasIncoming(rels, relType, e.ID) {
		return nil
	}
	return []ValidationIssue{archIssue("unresolved", SeverityMedium, e.ID, message)}
}

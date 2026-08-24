package validate

import (
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func validateMapping(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) ([]ValidationIssue, []PhaseSatisfaction) {
	checks := opts.Checks
	if len(checks) == 0 {
		checks = MappingChecks
	}

	var allIssues []ValidationIssue
	var reports []PhaseSatisfaction

	for _, check := range checks {
		layer, known := CheckLayer(check)
		if !known || layer != model.LayerMapping {
			continue
		}

		var issues []ValidationIssue
		switch check {
		case "plan_coverage":
			issues = checkPlanCoverage(opts, rf, ef)
		case "delivery_completeness":
			issues = checkDeliveryCompletenessFor(opts, rf, ef)
		case "mapping_consistency":
			issues = checkMappingConsistencyFor(opts, rf, ef)
		case "invalid_mapping_edges":
			issues = checkInvalidMappingEdgesFor(opts, rf, ef)
		case "gates":
			issues = checkGates(opts, rf, ef)
		case "task_scope":
			issues = checkTaskScopeFor(opts, rf, ef)
		case "phase_satisfaction":
			satIssues, satReports := checkPhaseSatisfaction(opts, rf, ef)
			issues = satIssues
			reports = append(reports, satReports...)
		}

		allIssues = append(allIssues, issues...)
	}

	return allIssues, reports
}

func isMappingRelation(r model.Relation) bool {
	return model.LayerForRelationType(r.Type) == model.LayerMapping
}

// mappingIssue builds a mapping-layer issue for the given check.
func mappingIssue(check string, severity Severity, entity, message string) ValidationIssue {
	return ValidationIssue{
		Check:    check,
		Severity: severity,
		Entity:   entity,
		Message:  message,
		Layer:    model.LayerMapping,
	}
}

// phaseScopeIssue reports a failure to derive a phase's effective scope. Without
// the scope a check has no subject set, so this cannot be silently skipped.
func phaseScopeIssue(check, phaseID string, err error) ValidationIssue {
	return mappingIssue(check, SeverityHigh, phaseID,
		fmt.Sprintf("could not derive scope for phase %s: %v", phaseID, err))
}

// subjectsIssue reports a failure to resolve the validation subject set.
func subjectsIssue(check string, err error) ValidationIssue {
	return mappingIssue(check, SeverityHigh, "",
		fmt.Sprintf("could not resolve validation subjects: %v", err))
}

func checkPlanCoverage(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	activePlan, issues := planUnderReview(opts, ef)
	if activePlan == nil {
		return issues
	}

	phases, err := execEntitiesOfType(ef, model.EntityTypePhase)
	if err != nil {
		return append(issues, listFailureIssue("plan_coverage", model.LayerMapping, err))
	}

	activeStatus := model.EntityStatusActive
	reqType := model.EntityTypeRequirement
	archLayer := model.LayerArch
	reqs, err := ef.List(EntityListFilters{Type: &reqType, Status: &activeStatus, Layer: &archLayer})
	if err != nil {
		return append(issues, listFailureIssue("plan_coverage", model.LayerMapping, err))
	}

	covered := make(map[string]bool)
	for _, p := range phases {
		rels, issue := fetchRelations(rf, p.ID, "plan_coverage", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		if parentPlanID(p.ID, rels) != activePlan.ID {
			continue
		}

		scope, scopeErr := graph.EffectivePhaseScope(p.ID, rf)
		if scopeErr != nil {
			issues = append(issues, phaseScopeIssue("plan_coverage", p.ID, scopeErr))
			continue
		}
		for _, coveredID := range scope.Covered {
			covered[coveredID] = true
		}
	}

	for _, req := range reqs {
		if !covered[req.ID] {
			issues = append(issues, mappingIssue("plan_coverage", SeverityHigh, req.ID,
				"active requirement not covered by any phase in the active plan"))
		}
	}

	return issues
}

// planUnderReview resolves the plan whose coverage is being checked: the one
// named in opts, or else the single active plan. A nil plan with no issues means
// there is no active plan to check.
func planUnderReview(opts ValidateOptions, ef EntityFetcher) (*model.Entity, []ValidationIssue) {
	if opts.Plan != nil {
		plan, issue := fetchEntity(ef, *opts.Plan, "plan_coverage", model.LayerMapping)
		if issue != nil {
			return nil, []ValidationIssue{*issue}
		}
		return plan, nil
	}

	planType := model.EntityTypePlan
	activeStatus := model.EntityStatusActive
	execLayer := model.LayerExec
	plans, err := ef.List(EntityListFilters{Type: &planType, Status: &activeStatus, Layer: &execLayer})
	if err != nil {
		return nil, []ValidationIssue{listFailureIssue("plan_coverage", model.LayerMapping, err)}
	}
	if len(plans) == 0 {
		return nil, nil
	}
	return &plans[0], nil
}

func checkDeliveryCompleteness(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	return checkDeliveryCompletenessFor(ValidateOptions{}, rf, ef)
}

func checkDeliveryCompletenessFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, issues := deliveryPhases(opts, ef)

	for _, phase := range phases {
		scope, err := graph.EffectivePhaseScope(phase.ID, rf)
		if err != nil {
			issues = append(issues, phaseScopeIssue("delivery_completeness", phase.ID, err))
			continue
		}

		delivered := make(map[string]bool, len(scope.Delivered))
		for _, id := range scope.Delivered {
			delivered[id] = true
		}

		for _, entityID := range scope.Covered {
			if delivered[entityID] {
				continue
			}
			entity, issue := fetchEntity(ef, entityID, "delivery_completeness", model.LayerMapping)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if entity == nil {
				continue
			}
			mappingLayer := model.LayerMapping
			if !model.IsEdgeAllowed(model.RelationDelivers, model.EntityTypePhase, entity.Type, &mappingLayer) {
				continue
			}
			issues = append(issues, mappingIssue("delivery_completeness", SeverityHigh, entityID,
				fmt.Sprintf("entity %s covered by resolved phase %s but not delivered", entityID, phase.ID)))
		}
	}

	return issues
}

// deliveryPhases returns the phases whose delivery completeness to check: the
// one named in opts, or else every resolved phase.
func deliveryPhases(opts ValidateOptions, ef EntityFetcher) ([]model.Entity, []ValidationIssue) {
	if opts.Phase != nil {
		phase, issue := fetchEntity(ef, *opts.Phase, "delivery_completeness", model.LayerMapping)
		if issue != nil {
			return nil, []ValidationIssue{*issue}
		}
		if phase == nil || phase.Type != model.EntityTypePhase {
			return nil, nil
		}
		return []model.Entity{*phase}, nil
	}

	phaseType := model.EntityTypePhase
	resolvedStatus := model.EntityStatusResolved
	execLayer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Status: &resolvedStatus, Layer: &execLayer})
	if err != nil {
		return nil, []ValidationIssue{listFailureIssue("delivery_completeness", model.LayerMapping, err)}
	}
	return phases, nil
}

func checkTaskScope(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	return checkTaskScopeFor(ValidateOptions{}, rf, ef)
}

func checkTaskScopeFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	subjects, err := resolveValidationSubjects(opts, rf, ef)
	if err != nil {
		return []ValidationIssue{subjectsIssue("task_scope", err)}
	}

	issues := checkTaskMappings(subjects, rf, ef)
	return append(issues, checkPhaseMappingStyle(subjects, rf, ef)...)
}

// checkTaskMappings requires every live task to cover what it works on, and to
// cover anything it delivers.
func checkTaskMappings(subjects validationSubjects, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	tasks, err := execEntitiesOfType(ef, model.EntityTypeTask)
	if err != nil {
		return []ValidationIssue{listFailureIssue("task_scope", model.LayerMapping, err)}
	}

	var issues []ValidationIssue
	for _, task := range tasks {
		if subjects.scoped && !subjects.taskIDs[task.ID] {
			continue
		}
		relations, issue := fetchRelations(rf, task.ID, "task_scope", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		covered := make(map[string]bool)
		for _, relation := range relations {
			if relation.FromID == task.ID && relation.Type == model.RelationCovers {
				covered[relation.ToID] = true
			}
		}

		if task.Status != model.EntityStatusDeprecated && len(covered) == 0 {
			issues = append(issues, mappingIssue("task_scope", SeverityHigh, task.ID,
				"non-deprecated task must cover at least one architecture entity"))
		}

		for _, relation := range relations {
			if relation.FromID != task.ID || relation.Type != model.RelationDelivers {
				continue
			}
			if covered[relation.ToID] {
				continue
			}
			target, issue := fetchEntity(ef, relation.ToID, "task_scope", model.LayerMapping)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if target == nil {
				continue
			}
			mappingLayer := model.LayerMapping
			if model.IsEdgeAllowed(model.RelationDelivers, model.EntityTypeTask, target.Type, &mappingLayer) {
				issues = append(issues, mappingIssue("task_scope", SeverityHigh, task.ID,
					fmt.Sprintf("task delivers %s without covering it", relation.ToID)))
			}
		}
	}

	return issues
}

// checkPhaseMappingStyle rejects a phase that maps entities both directly and
// through its child tasks, since the two styles disagree about scope ownership.
func checkPhaseMappingStyle(subjects validationSubjects, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, err := execEntitiesOfType(ef, model.EntityTypePhase)
	if err != nil {
		return []ValidationIssue{listFailureIssue("task_scope", model.LayerMapping, err)}
	}

	var issues []ValidationIssue
	for _, phase := range phases {
		if subjects.scoped && !subjects.phaseIDs[phase.ID] {
			continue
		}
		scope, scopeErr := graph.EffectivePhaseScope(phase.ID, rf)
		if scopeErr != nil {
			issues = append(issues, phaseScopeIssue("task_scope", phase.ID, scopeErr))
			continue
		}
		if !scope.TaskManaged || len(scope.Relations) == 0 {
			continue
		}

		phaseRelations, issue := fetchRelations(rf, phase.ID, "task_scope", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		for _, relation := range phaseRelations {
			if relation.FromID != phase.ID {
				continue
			}
			if relation.Type == model.RelationCovers || relation.Type == model.RelationDelivers {
				issues = append(issues, mappingIssue("task_scope", SeverityHigh, phase.ID,
					"phase has mixed direct phase and child-task mappings"))
				break
			}
		}
	}

	return issues
}

func checkMappingConsistency(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	return checkMappingConsistencyFor(ValidateOptions{}, rf, ef)
}

func checkMappingConsistencyFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	subjects, err := resolveValidationSubjects(opts, rf, ef)
	if err != nil {
		return []ValidationIssue{subjectsIssue("mapping_consistency", err)}
	}

	entities, issues := mappingSubjects(subjects, ef, "mapping_consistency", false)

	seen := make(map[string]bool)
	for _, e := range entities {
		// A resolved phase or task records execution that already finished, so
		// its mappings must keep pointing at the entity revision that was
		// actually delivered even after that revision is deprecated.
		if e.Status == model.EntityStatusResolved {
			continue
		}

		rels, issue := fetchRelations(rf, e.ID, "mapping_consistency", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		for _, r := range rels {
			if r.FromID != e.ID || !isMappingRelation(r) {
				continue
			}
			if r.Type != model.RelationCovers && r.Type != model.RelationDelivers {
				continue
			}

			key := fmt.Sprintf("%d|%s|%s|%s", r.ID, r.FromID, r.ToID, r.Type)
			if seen[key] {
				continue
			}
			seen[key] = true

			issues = append(issues, staleMappingTargetIssues(r, rf, ef)...)
		}
	}

	return issues
}

// staleMappingTargetIssues reports a mapping relation aimed at an entity
// revision that is no longer current.
func staleMappingTargetIssues(r model.Relation, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	archEntityID := r.ToID

	archEntity, issue := fetchEntity(ef, archEntityID, "mapping_consistency", model.LayerMapping)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	if archEntity == nil {
		return nil
	}

	var issues []ValidationIssue
	if archEntity.Status == model.EntityStatusDeprecated {
		issues = append(issues, mappingIssue("mapping_consistency", SeverityMedium, archEntityID,
			fmt.Sprintf("mapping relation %q targets deprecated entity", r.Type)))
	}

	archRels, issue := fetchRelations(rf, archEntityID, "mapping_consistency", model.LayerMapping)
	if issue != nil {
		return append(issues, *issue)
	}
	for _, ar := range archRels {
		if ar.Type == model.RelationSupersedes && ar.ToID == archEntityID {
			issues = append(issues, mappingIssue("mapping_consistency", SeverityMedium, archEntityID,
				fmt.Sprintf("mapping relation %q targets superseded entity (superseded by %s)", r.Type, ar.FromID)))
			break
		}
	}

	return issues
}

func checkInvalidMappingEdges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	return checkInvalidMappingEdgesFor(ValidateOptions{}, rf, ef)
}

func checkInvalidMappingEdgesFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	subjects, err := resolveValidationSubjects(opts, rf, ef)
	if err != nil {
		return []ValidationIssue{subjectsIssue("invalid_mapping_edges", err)}
	}

	allEntities, issues := mappingSubjects(subjects, ef, "invalid_mapping_edges", true)

	seen := make(map[string]bool)
	for _, e := range allEntities {
		rels, issue := fetchRelations(rf, e.ID, "invalid_mapping_edges", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		for _, rel := range rels {
			if !isMappingRelation(rel) {
				continue
			}

			key := fmt.Sprintf("%s|%s|%s", rel.FromID, rel.ToID, rel.Type)
			if seen[key] {
				continue
			}
			seen[key] = true

			issues = append(issues, mappingEdgeIssues(rel, subjects, ef)...)
		}
	}

	return issues
}

// mappingSubjects returns the entities whose mapping relations to inspect. When
// unscoped, includeArch adds the arch layer so mapping edges are seen from both
// endpoints.
func mappingSubjects(subjects validationSubjects, ef EntityFetcher, check string, includeArch bool) ([]model.Entity, []ValidationIssue) {
	if subjects.scoped {
		var entities []model.Entity
		var issues []ValidationIssue
		for id := range subjects.execIDs {
			entity, issue := fetchEntity(ef, id, check, model.LayerMapping)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
		return entities, issues
	}

	var entities []model.Entity
	var issues []ValidationIssue

	execEnts, err := execEntities(ef)
	if err != nil {
		issues = append(issues, listFailureIssue(check, model.LayerMapping, err))
	} else {
		entities = append(entities, execEnts...)
	}

	if includeArch {
		archEnts, err := archEntities(ef)
		if err != nil {
			issues = append(issues, listFailureIssue(check, model.LayerMapping, err))
		} else {
			entities = append(entities, archEnts...)
		}
	}

	return entities, issues
}

// mappingEdgeIssues reports a relation whose endpoints the mapping edge matrix
// forbids.
func mappingEdgeIssues(rel model.Relation, subjects validationSubjects, ef EntityFetcher) []ValidationIssue {
	srcEntity, issue := fetchEntity(ef, rel.FromID, "invalid_mapping_edges", model.LayerMapping)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	tgtEntity, issue := fetchEntity(ef, rel.ToID, "invalid_mapping_edges", model.LayerMapping)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	if srcEntity == nil || tgtEntity == nil {
		return nil
	}

	mappingLayer := model.LayerMapping
	if model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, &mappingLayer) {
		return nil
	}

	// Attribute the issue to whichever endpoint is in scope, so a scoped run
	// does not filter out its own finding.
	issueEntity := rel.FromID
	if subjects.scoped && !subjects.execIDs[issueEntity] {
		issueEntity = rel.ToID
	}
	return []ValidationIssue{mappingIssue("invalid_mapping_edges", SeverityHigh, issueEntity,
		fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type))}
}

func checkGates(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, issues := gatePhases(opts, ef)

	for _, phase := range phases {
		scope, scopeErr := graph.EffectivePhaseScope(phase.ID, rf)
		if scopeErr != nil {
			issues = append(issues, phaseScopeIssue("gates", phase.ID, scopeErr))
			continue
		}

		for _, entityID := range scope.Covered {
			entity, issue := fetchEntity(ef, entityID, "gates", model.LayerMapping)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			if entity == nil {
				continue
			}
			if entity.Status != model.EntityStatusActive && entity.Status != model.EntityStatusDraft {
				continue
			}

			eRels, issue := fetchRelations(rf, entityID, "gates", model.LayerMapping)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}

			issues = append(issues, unresolvedGateIssues(*entity, phase.ID, eRels)...)
			issues = append(issues, draftDecisionGateIssues(*entity, phase.ID, eRels, ef)...)
		}
	}

	return issues
}

// gatePhases returns the phases to gate: the one named in opts, or else every
// active phase.
func gatePhases(opts ValidateOptions, ef EntityFetcher) ([]model.Entity, []ValidationIssue) {
	if opts.Phase != nil {
		phase, issue := fetchEntity(ef, *opts.Phase, "gates", model.LayerMapping)
		if issue != nil {
			return nil, []ValidationIssue{*issue}
		}
		if phase == nil {
			return nil, nil
		}
		return []model.Entity{*phase}, nil
	}

	phaseType := model.EntityTypePhase
	activeStatus := model.EntityStatusActive
	execLayer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Status: &activeStatus, Layer: &execLayer})
	if err != nil {
		return nil, []ValidationIssue{listFailureIssue("gates", model.LayerMapping, err)}
	}
	return phases, nil
}

// unresolvedGateIssues reports open questions, unmitigated risks, and unverified
// assumptions sitting in a phase's scope.
func unresolvedGateIssues(entity model.Entity, phaseID string, rels []model.Relation) []ValidationIssue {
	switch entity.Type {
	case model.EntityTypeQuestion:
		if hasIncoming(rels, model.RelationAnswers, entity.ID) {
			return nil
		}
		return []ValidationIssue{mappingIssue("gates", SeverityHigh, entity.ID,
			fmt.Sprintf("unresolved question in phase %s scope", phaseID))}

	case model.EntityTypeRisk:
		if hasIncoming(rels, model.RelationMitigates, entity.ID) {
			return nil
		}
		return []ValidationIssue{mappingIssue("gates", SeverityHigh, entity.ID,
			fmt.Sprintf("unmitigated risk in phase %s scope", phaseID))}

	case model.EntityTypeAssumption:
		return []ValidationIssue{mappingIssue("gates", SeverityMedium, entity.ID,
			fmt.Sprintf("unverified assumption in phase %s scope", phaseID))}

	default:
		return nil
	}
}

// draftDecisionGateIssues reports a requirement depending on a decision that has
// not been made yet.
func draftDecisionGateIssues(entity model.Entity, phaseID string, rels []model.Relation, ef EntityFetcher) []ValidationIssue {
	if entity.Type != model.EntityTypeRequirement {
		return nil
	}

	var issues []ValidationIssue
	for _, r := range rels {
		if r.Type != model.RelationDependsOn || r.FromID != entity.ID {
			continue
		}
		dep, issue := fetchEntity(ef, r.ToID, "gates", model.LayerMapping)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		if dep == nil {
			continue
		}
		if dep.Type == model.EntityTypeDecision && dep.Status == model.EntityStatusDraft {
			issues = append(issues, mappingIssue("gates", SeverityHigh, entity.ID,
				fmt.Sprintf("depends on draft decision %s in phase %s scope", dep.ID, phaseID)))
		}
	}

	return issues
}

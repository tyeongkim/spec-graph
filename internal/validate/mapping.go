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
			issues = checkDeliveryCompleteness(rf, ef)
		case "mapping_consistency":
			issues = checkMappingConsistency(rf, ef)
		case "invalid_mapping_edges":
			issues = checkInvalidMappingEdges(rf, ef)
		case "gates":
			issues = checkGates(opts, rf, ef)
		case "task_scope":
			issues = checkTaskScope(rf, ef)
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

func checkPlanCoverage(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	planType := model.EntityTypePlan
	activeStatus := model.EntityStatusActive
	execLayer := model.LayerExec
	var activePlan model.Entity
	if opts.Plan != nil {
		plan, err := ef.Get(*opts.Plan)
		if err != nil {
			return nil
		}
		activePlan = plan
	} else {
		plans, err := ef.List(EntityListFilters{Type: &planType, Status: &activeStatus, Layer: &execLayer})
		if err != nil || len(plans) == 0 {
			return nil
		}
		activePlan = plans[0]
	}

	planPhaseIDs := make(map[string]bool)
	phaseType := model.EntityTypePhase
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Layer: &execLayer})
	if err != nil {
		return nil
	}
	for _, p := range phases {
		rels, err := rf.GetByEntity(p.ID)
		if err != nil {
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationBelongsTo && r.FromID == p.ID && r.ToID == activePlan.ID {
				planPhaseIDs[p.ID] = true
				break
			}
		}
	}

	reqType := model.EntityTypeRequirement
	archLayer := model.LayerArch
	reqs, err := ef.List(EntityListFilters{Type: &reqType, Status: &activeStatus, Layer: &archLayer})
	if err != nil {
		return nil
	}

	coveredRequirements := make(map[string]bool)
	for phaseID := range planPhaseIDs {
		scope, scopeErr := graph.EffectivePhaseScope(phaseID, rf)
		if scopeErr != nil {
			continue
		}
		for _, coveredID := range scope.Covered {
			coveredRequirements[coveredID] = true
		}
	}

	var issues []ValidationIssue
	for _, req := range reqs {
		if !coveredRequirements[req.ID] {
			issues = append(issues, ValidationIssue{
				Check:    "plan_coverage",
				Severity: SeverityHigh,
				Entity:   req.ID,
				Message:  "active requirement not covered by any phase in the active plan",
				Layer:    model.LayerMapping,
			})
		}
	}

	return issues
}

func checkDeliveryCompleteness(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phaseType := model.EntityTypePhase
	resolvedStatus := model.EntityStatusResolved
	execLayer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Status: &resolvedStatus, Layer: &execLayer})
	if err != nil {
		return nil
	}

	var issues []ValidationIssue

	for _, phase := range phases {
		scope, err := graph.EffectivePhaseScope(phase.ID, rf)
		if err != nil {
			continue
		}

		coveredEntities := make(map[string]bool)
		for _, id := range scope.Covered {
			coveredEntities[id] = true
		}

		deliveredEntities := make(map[string]bool)
		for _, id := range scope.Delivered {
			deliveredEntities[id] = true
		}

		for entityID := range coveredEntities {
			entity, err := ef.Get(entityID)
			if err != nil {
				continue
			}
			mappingLayer := model.LayerMapping
			if !model.IsEdgeAllowed(model.RelationDelivers, model.EntityTypePhase, entity.Type, &mappingLayer) {
				continue
			}
			if !deliveredEntities[entityID] {
				issues = append(issues, ValidationIssue{
					Check:    "delivery_completeness",
					Severity: SeverityHigh,
					Entity:   entityID,
					Message:  fmt.Sprintf("entity %s covered by resolved phase %s but not delivered", entityID, phase.ID),
					Layer:    model.LayerMapping,
				})
			}
		}
	}

	return issues
}

func checkTaskScope(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	taskType := model.EntityTypeTask
	execLayer := model.LayerExec
	tasks, err := ef.List(EntityListFilters{Type: &taskType, Layer: &execLayer})
	if err != nil {
		return nil
	}

	var issues []ValidationIssue
	for _, task := range tasks {
		relations, fetchErr := rf.GetByEntity(task.ID)
		if fetchErr != nil {
			continue
		}
		covered := make(map[string]bool)
		for _, relation := range relations {
			if relation.FromID != task.ID {
				continue
			}
			switch relation.Type {
			case model.RelationCovers:
				covered[relation.ToID] = true
			}
		}
		if task.Status != model.EntityStatusDeprecated && len(covered) == 0 {
			issues = append(issues, ValidationIssue{Check: "task_scope", Severity: SeverityHigh, Entity: task.ID, Message: "non-deprecated task must cover at least one architecture entity", Layer: model.LayerMapping})
		}
		for _, relation := range relations {
			if relation.FromID != task.ID || relation.Type != model.RelationDelivers {
				continue
			}
			target, targetErr := ef.Get(relation.ToID)
			if targetErr != nil {
				continue
			}
			mappingLayer := model.LayerMapping
			if model.IsEdgeAllowed(model.RelationDelivers, model.EntityTypeTask, target.Type, &mappingLayer) && !covered[relation.ToID] {
				issues = append(issues, ValidationIssue{Check: "task_scope", Severity: SeverityHigh, Entity: task.ID, Message: fmt.Sprintf("task delivers %s without covering it", relation.ToID), Layer: model.LayerMapping})
			}
		}
	}

	phaseType := model.EntityTypePhase
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Layer: &execLayer})
	if err != nil {
		return issues
	}
	for _, phase := range phases {
		scope, scopeErr := graph.EffectivePhaseScope(phase.ID, rf)
		if scopeErr != nil || !scope.TaskManaged {
			continue
		}
		phaseRelations, fetchErr := rf.GetByEntity(phase.ID)
		if fetchErr != nil {
			continue
		}
		hasDirectMappings := false
		for _, relation := range phaseRelations {
			if relation.FromID == phase.ID && (relation.Type == model.RelationCovers || relation.Type == model.RelationDelivers) {
				hasDirectMappings = true
				break
			}
		}
		if hasDirectMappings && len(scope.Relations) > 0 {
			issues = append(issues, ValidationIssue{Check: "task_scope", Severity: SeverityHigh, Entity: phase.ID, Message: "phase has mixed direct phase and child-task mappings", Layer: model.LayerMapping})
		}
	}
	return issues
}

func checkMappingConsistency(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	entities, err := execEntities(ef)
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

		for _, r := range rels {
			if !isMappingRelation(r) {
				continue
			}

			key := fmt.Sprintf("%d|%s|%s|%s", r.ID, r.FromID, r.ToID, r.Type)
			if seen[key] {
				continue
			}
			seen[key] = true

			var archEntityID string
			switch r.Type {
			case model.RelationCovers, model.RelationDelivers:
				archEntityID = r.ToID
			default:
				continue
			}

			archEntity, err := ef.Get(archEntityID)
			if err != nil {
				continue
			}

			if archEntity.Status == model.EntityStatusDeprecated {
				issues = append(issues, ValidationIssue{
					Check:    "mapping_consistency",
					Severity: SeverityMedium,
					Entity:   archEntityID,
					Message:  fmt.Sprintf("mapping relation %q targets deprecated entity", r.Type),
					Layer:    model.LayerMapping,
				})
			}

			archRels, err := rf.GetByEntity(archEntityID)
			if err != nil {
				continue
			}
			for _, ar := range archRels {
				if ar.Type == model.RelationSupersedes && ar.ToID == archEntityID {
					issues = append(issues, ValidationIssue{
						Check:    "mapping_consistency",
						Severity: SeverityMedium,
						Entity:   archEntityID,
						Message:  fmt.Sprintf("mapping relation %q targets superseded entity (superseded by %s)", r.Type, ar.FromID),
						Layer:    model.LayerMapping,
					})
					break
				}
			}
		}
	}

	return issues
}

func checkInvalidMappingEdges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	var allEntities []model.Entity

	execEnts, err := execEntities(ef)
	if err == nil {
		allEntities = append(allEntities, execEnts...)
	}
	archEnts, err := archEntities(ef)
	if err == nil {
		allEntities = append(allEntities, archEnts...)
	}

	seen := make(map[string]bool)
	var issues []ValidationIssue

	for _, e := range allEntities {
		rels, err := rf.GetByEntity(e.ID)
		if err != nil {
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

			srcEntity, err := ef.Get(rel.FromID)
			if err != nil {
				continue
			}
			tgtEntity, err := ef.Get(rel.ToID)
			if err != nil {
				continue
			}

			mappingLayer := model.LayerMapping
			if !model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, &mappingLayer) {
				issues = append(issues, ValidationIssue{
					Check:    "invalid_mapping_edges",
					Severity: SeverityHigh,
					Entity:   rel.FromID,
					Message:  fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type),
					Layer:    model.LayerMapping,
				})
			}
		}
	}

	return issues
}

func checkGates(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phaseType := model.EntityTypePhase
	activeStatus := model.EntityStatusActive
	execLayer := model.LayerExec

	var phases []model.Entity
	if opts.Phase != nil {
		p, err := ef.Get(*opts.Phase)
		if err != nil {
			return nil
		}
		phases = []model.Entity{p}
	} else {
		var err error
		phases, err = ef.List(EntityListFilters{Type: &phaseType, Status: &activeStatus, Layer: &execLayer})
		if err != nil {
			return nil
		}
	}

	var issues []ValidationIssue

	for _, phase := range phases {
		scope, scopeErr := graph.EffectivePhaseScope(phase.ID, rf)
		if scopeErr != nil {
			continue
		}
		coveredIDs := make(map[string]bool, len(scope.Covered))
		for _, id := range scope.Covered {
			coveredIDs[id] = true
		}

		for entityID := range coveredIDs {
			entity, err := ef.Get(entityID)
			if err != nil {
				continue
			}
			if entity.Status != model.EntityStatusActive && entity.Status != model.EntityStatusDraft {
				continue
			}

			eRels, err := rf.GetByEntity(entityID)
			if err != nil {
				continue
			}

			switch entity.Type {
			case model.EntityTypeQuestion:
				hasAnswer := false
				for _, r := range eRels {
					if r.Type == model.RelationAnswers && r.ToID == entityID {
						hasAnswer = true
						break
					}
				}
				if !hasAnswer {
					issues = append(issues, ValidationIssue{
						Check:    "gates",
						Severity: SeverityHigh,
						Entity:   entityID,
						Message:  fmt.Sprintf("unresolved question in phase %s scope", phase.ID),
						Layer:    model.LayerMapping,
					})
				}

			case model.EntityTypeRisk:
				hasMitigation := false
				for _, r := range eRels {
					if r.Type == model.RelationMitigates && r.ToID == entityID {
						hasMitigation = true
						break
					}
				}
				if !hasMitigation {
					issues = append(issues, ValidationIssue{
						Check:    "gates",
						Severity: SeverityHigh,
						Entity:   entityID,
						Message:  fmt.Sprintf("unmitigated risk in phase %s scope", phase.ID),
						Layer:    model.LayerMapping,
					})
				}

			case model.EntityTypeAssumption:
				issues = append(issues, ValidationIssue{
					Check:    "gates",
					Severity: SeverityMedium,
					Entity:   entityID,
					Message:  fmt.Sprintf("unverified assumption in phase %s scope", phase.ID),
					Layer:    model.LayerMapping,
				})
			}

			if entity.Type == model.EntityTypeRequirement {
				for _, r := range eRels {
					if r.Type != model.RelationDependsOn || r.FromID != entityID {
						continue
					}
					dep, err := ef.Get(r.ToID)
					if err != nil {
						continue
					}
					if dep.Type == model.EntityTypeDecision && dep.Status == model.EntityStatusDraft {
						issues = append(issues, ValidationIssue{
							Check:    "gates",
							Severity: SeverityHigh,
							Entity:   entityID,
							Message:  fmt.Sprintf("depends on draft decision %s in phase %s scope", dep.ID, phase.ID),
							Layer:    model.LayerMapping,
						})
					}
				}
			}
		}
	}

	return issues
}

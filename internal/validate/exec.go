package validate

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func validateExec(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	checks := opts.Checks
	if len(checks) == 0 {
		checks = ExecChecks
	}

	var allIssues []ValidationIssue

	for _, check := range checks {
		layer, known := CheckLayer(check)
		if !known || layer != model.LayerExec {
			continue
		}

		var issues []ValidationIssue
		switch check {
		case "phase_order":
			issues = checkPhaseOrder(rf, ef)
		case "single_active_plan":
			issues = checkSingleActivePlan(ef)
		case "orphan_phases":
			issues = checkOrphanPhases(rf, ef)
		case "exec_cycles":
			issues = checkExecCycles(rf, ef)
		case "invalid_exec_edges":
			issues = checkInvalidExecEdges(rf, ef)
		case "orphan_changes":
			issues = checkOrphanChanges(rf, ef)
		case "task_graph":
			issues = checkTaskGraph(rf, ef)
		}

		allIssues = append(allIssues, issues...)
	}

	return allIssues
}

func checkTaskGraph(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	taskType := model.EntityTypeTask
	layer := model.LayerExec
	tasks, err := ef.List(EntityListFilters{Type: &taskType, Layer: &layer})
	if err != nil {
		return nil
	}

	taskByID := make(map[string]model.Entity, len(tasks))
	parents := make(map[string][]string, len(tasks))
	dependencies := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		taskByID[task.ID] = task
		relations, getErr := rf.GetByEntity(task.ID)
		if getErr != nil {
			continue
		}
		for _, relation := range relations {
			if relation.FromID != task.ID {
				continue
			}
			switch relation.Type {
			case model.RelationBelongsTo:
				parents[task.ID] = append(parents[task.ID], relation.ToID)
			case model.RelationTaskDependsOn:
				dependencies[task.ID] = append(dependencies[task.ID], relation.ToID)
			}
		}
	}

	var issues []ValidationIssue
	addIssue := func(entity, message string) {
		issues = append(issues, ValidationIssue{Check: "task_graph", Severity: SeverityHigh, Entity: entity, Message: message, Layer: model.LayerExec})
	}
	for _, task := range tasks {
		taskParents := parents[task.ID]
		sort.Strings(taskParents)
		switch len(taskParents) {
		case 0:
			addIssue(task.ID, fmt.Sprintf("task %s has zero parent phases", task.ID))
		case 1:
		default:
			addIssue(task.ID, fmt.Sprintf("task %s has multiple parent phases: %s", task.ID, strings.Join(taskParents, ", ")))
		}

		for _, prerequisiteID := range dependencies[task.ID] {
			if prerequisiteID == task.ID {
				addIssue(task.ID, fmt.Sprintf("task %s has self-dependency %s -> %s", task.ID, task.ID, prerequisiteID))
				continue
			}
			prerequisite, ok := taskByID[prerequisiteID]
			if !ok {
				continue
			}
			if prerequisite.Status == model.EntityStatusDeprecated {
				addIssue(task.ID, fmt.Sprintf("task %s depends on deprecated task %s", task.ID, prerequisiteID))
			}
			if len(taskParents) == 1 && len(parents[prerequisiteID]) == 1 && taskParents[0] != parents[prerequisiteID][0] {
				addIssue(task.ID, fmt.Sprintf("cross-phase task dependency %s (%s) -> %s (%s)", task.ID, taskParents[0], prerequisiteID, parents[prerequisiteID][0]))
			}
		}
	}

	visited := make(map[string]bool, len(tasks))
	inStack := make(map[string]bool, len(tasks))
	var stack []string
	var visit func(string)
	visit = func(taskID string) {
		visited[taskID] = true
		inStack[taskID] = true
		stack = append(stack, taskID)
		for _, prerequisiteID := range dependencies[taskID] {
			if prerequisiteID == taskID {
				continue
			}
			if _, ok := taskByID[prerequisiteID]; !ok {
				continue
			}
			if !visited[prerequisiteID] {
				visit(prerequisiteID)
				continue
			}
			if inStack[prerequisiteID] {
				start := slices.Index(stack, prerequisiteID)
				members := append([]string(nil), stack[start:]...)
				description := strings.Join(append(append([]string(nil), members...), prerequisiteID), " -> ")
				for _, member := range members {
					addIssue(member, fmt.Sprintf("task dependency cycle members [%s]: %s", strings.Join(members, ", "), description))
				}
			}
		}
		stack = stack[:len(stack)-1]
		inStack[taskID] = false
	}
	for _, task := range tasks {
		if !visited[task.ID] {
			visit(task.ID)
		}
	}

	return issues
}

func execEntities(ef EntityFetcher) ([]model.Entity, error) {
	layer := model.LayerExec
	return ef.List(EntityListFilters{Layer: &layer})
}

func isExecRelation(r model.Relation) bool {
	return model.LayerForRelationType(r.Type) == model.LayerExec
}

// checkPhaseOrder detects duplicate order values among phases within the same plan.
func checkPhaseOrder(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phaseType := model.EntityTypePhase
	layer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Layer: &layer})
	if err != nil {
		return nil
	}

	phasePlan := make(map[string]string)
	for _, p := range phases {
		rels, err := rf.GetByEntity(p.ID)
		if err != nil {
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationBelongsTo && r.FromID == p.ID {
				phasePlan[p.ID] = r.ToID
				break
			}
		}
	}

	type phaseOrder struct {
		id    string
		order int
	}
	planPhases := make(map[string][]phaseOrder)

	for _, p := range phases {
		planID, ok := phasePlan[p.ID]
		if !ok {
			continue
		}

		var meta struct {
			Order *int `json:"order"`
		}
		if err := json.Unmarshal(p.Metadata, &meta); err != nil || meta.Order == nil {
			continue
		}

		planPhases[planID] = append(planPhases[planID], phaseOrder{id: p.ID, order: *meta.Order})
	}

	var issues []ValidationIssue
	for _, phs := range planPhases {
		orderSeen := make(map[int][]string)
		for _, po := range phs {
			orderSeen[po.order] = append(orderSeen[po.order], po.id)
		}
		for ord, ids := range orderSeen {
			if len(ids) > 1 {
				for _, id := range ids {
					issues = append(issues, ValidationIssue{
						Check:    "phase_order",
						Severity: SeverityHigh,
						Entity:   id,
						Message:  fmt.Sprintf("duplicate phase order %d", ord),
						Layer:    model.LayerExec,
					})
				}
			}
		}
	}

	return issues
}

// checkSingleActivePlan reports when more than one plan has status=active.
func checkSingleActivePlan(ef EntityFetcher) []ValidationIssue {
	planType := model.EntityTypePlan
	activeStatus := model.EntityStatusActive
	layer := model.LayerExec
	plans, err := ef.List(EntityListFilters{Type: &planType, Status: &activeStatus, Layer: &layer})
	if err != nil {
		return nil
	}

	if len(plans) <= 1 {
		return nil
	}

	issues := make([]ValidationIssue, 0, len(plans))
	for _, p := range plans {
		issues = append(issues, ValidationIssue{
			Check:    "single_active_plan",
			Severity: SeverityHigh,
			Entity:   p.ID,
			Message:  fmt.Sprintf("multiple active plans detected (%d total)", len(plans)),
			Layer:    model.LayerExec,
		})
	}

	return issues
}

// checkOrphanPhases finds phases that have no belongs_to relation to any plan.
func checkOrphanPhases(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phaseType := model.EntityTypePhase
	layer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Layer: &layer})
	if err != nil {
		return nil
	}

	var issues []ValidationIssue
	for _, p := range phases {
		rels, err := rf.GetByEntity(p.ID)
		if err != nil {
			continue
		}

		hasBelongsTo := false
		for _, r := range rels {
			if r.Type == model.RelationBelongsTo && r.FromID == p.ID {
				hasBelongsTo = true
				break
			}
		}

		if !hasBelongsTo {
			issues = append(issues, ValidationIssue{
				Check:    "orphan_phases",
				Severity: SeverityMedium,
				Entity:   p.ID,
				Message:  "phase does not belong to any plan",
				Layer:    model.LayerExec,
			})
		}
	}

	return issues
}

// checkExecCycles detects circular blocks relations between phases using DFS.
func checkExecCycles(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phaseType := model.EntityTypePhase
	layer := model.LayerExec
	phases, err := ef.List(EntityListFilters{Type: &phaseType, Layer: &layer})
	if err != nil {
		return nil
	}

	phaseIDs := make(map[string]bool, len(phases))
	adj := make(map[string][]string)
	for _, p := range phases {
		phaseIDs[p.ID] = true
		rels, err := rf.GetByEntity(p.ID)
		if err != nil {
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationBlocks && r.FromID == p.ID {
				adj[p.ID] = append(adj[p.ID], r.ToID)
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
			if !phaseIDs[next] {
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
						Check:    "exec_cycles",
						Severity: SeverityHigh,
						Entity:   id,
						Message:  fmt.Sprintf("circular blocks dependency detected: %s", cycleDesc),
						Layer:    model.LayerExec,
					})
				}
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, p := range phases {
		if !visited[p.ID] {
			dfs(p.ID, nil)
		}
	}

	return issues
}

// checkInvalidExecEdges finds exec-layer relations that violate the exec edge matrix.
// Note: CHG is intentionally absent from execEdgeMatrix; invalid CHG exec edges are caught automatically if added.
func checkInvalidExecEdges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
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

		for _, rel := range rels {
			if !isExecRelation(rel) {
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

			execLayer := model.LayerExec
			if !model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, &execLayer) {
				issues = append(issues, ValidationIssue{
					Check:    "invalid_exec_edges",
					Severity: SeverityHigh,
					Entity:   rel.FromID,
					Message:  fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type),
					Layer:    model.LayerExec,
				})
			}
		}
	}

	return issues
}

// checkOrphanChanges finds CHG entities that have no relation to any non-CHG entity.
func checkOrphanChanges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	changeType := model.EntityTypeChange
	layer := model.LayerExec
	changes, err := ef.List(EntityListFilters{Type: &changeType, Layer: &layer})
	if err != nil {
		return nil
	}

	var issues []ValidationIssue
	for _, chg := range changes {
		rels, err := rf.GetByEntity(chg.ID)
		if err != nil {
			continue
		}

		hasNonCHGRelation := false
		for _, r := range rels {
			otherID := r.ToID
			if otherID == chg.ID {
				otherID = r.FromID
			}
			other, err := ef.Get(otherID)
			if err != nil {
				hasNonCHGRelation = true
				break
			}
			if other.Type != model.EntityTypeChange {
				hasNonCHGRelation = true
				break
			}
		}

		if !hasNonCHGRelation {
			severity := SeverityMedium
			if chg.Status == model.EntityStatusActive || chg.Status == model.EntityStatusResolved || chg.Status == model.EntityStatusDeprecated {
				severity = SeverityHigh
			}
			issues = append(issues, ValidationIssue{
				Check:    "orphan_changes",
				Severity: severity,
				Entity:   chg.ID,
				Message:  "change has no relations to other entities",
				Layer:    model.LayerExec,
			})
		}
	}

	return issues
}

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
			issues = checkInvalidExecEdgesFor(opts, rf, ef)
		case "orphan_changes":
			issues = checkOrphanChanges(rf, ef)
		case "task_graph":
			issues = checkTaskGraphFor(opts, rf, ef)
		}

		allIssues = append(allIssues, issues...)
	}

	return allIssues
}

func execEntities(ef EntityFetcher) ([]model.Entity, error) {
	layer := model.LayerExec
	return ef.List(EntityListFilters{Layer: &layer})
}

func execEntitiesOfType(ef EntityFetcher, entityType model.EntityType) ([]model.Entity, error) {
	layer := model.LayerExec
	return ef.List(EntityListFilters{Type: &entityType, Layer: &layer})
}

func isExecRelation(r model.Relation) bool {
	return model.LayerForRelationType(r.Type) == model.LayerExec
}

// execIssue builds an exec-layer issue for the given check.
func execIssue(check string, severity Severity, entity, message string) ValidationIssue {
	return ValidationIssue{
		Check:    check,
		Severity: severity,
		Entity:   entity,
		Message:  message,
		Layer:    model.LayerExec,
	}
}

func checkTaskGraph(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	return checkTaskGraphFor(ValidateOptions{}, rf, ef)
}

// taskGraph is the parent and prerequisite structure of the task set.
type taskGraph struct {
	byID         map[string]model.Entity
	parents      map[string][]string
	dependencies map[string][]string
}

func checkTaskGraphFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	subjects, err := resolveValidationSubjects(opts, rf, ef)
	if err != nil {
		return []ValidationIssue{execIssue("task_graph", SeverityHigh, "",
			fmt.Sprintf("could not resolve validation subjects: %v", err))}
	}

	tasks, err := execEntitiesOfType(ef, model.EntityTypeTask)
	if err != nil {
		return []ValidationIssue{listFailureIssue("task_graph", model.LayerExec, err)}
	}

	graph, issues := buildTaskGraph(tasks, rf)

	// A task_graph issue is reported only for tasks in scope, but the graph is
	// always built from every task, since cycles and cross-phase dependencies
	// can run through tasks outside the scope.
	record := func(entity, message string) {
		if subjects.scoped && !subjects.taskIDs[entity] {
			return
		}
		issues = append(issues, execIssue("task_graph", SeverityHigh, entity, message))
	}

	for _, task := range tasks {
		checkTaskParentage(task, graph, record)
		checkTaskDependencies(task, graph, record)
	}
	findTaskCycles(tasks, graph, record)

	return issues
}

// buildTaskGraph indexes each task's parent phases and prerequisite tasks.
func buildTaskGraph(tasks []model.Entity, rf RelationFetcher) (taskGraph, []ValidationIssue) {
	graph := taskGraph{
		byID:         make(map[string]model.Entity, len(tasks)),
		parents:      make(map[string][]string, len(tasks)),
		dependencies: make(map[string][]string, len(tasks)),
	}
	var issues []ValidationIssue

	for _, task := range tasks {
		graph.byID[task.ID] = task
		relations, issue := fetchRelations(rf, task.ID, "task_graph", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		for _, relation := range relations {
			if relation.FromID != task.ID {
				continue
			}
			switch relation.Type {
			case model.RelationBelongsTo:
				graph.parents[task.ID] = append(graph.parents[task.ID], relation.ToID)
			case model.RelationTaskDependsOn:
				graph.dependencies[task.ID] = append(graph.dependencies[task.ID], relation.ToID)
			}
		}
		sort.Strings(graph.parents[task.ID])
	}

	return graph, issues
}

// checkTaskParentage requires every task to belong to exactly one phase.
func checkTaskParentage(task model.Entity, graph taskGraph, record func(entity, message string)) {
	parents := graph.parents[task.ID]
	switch len(parents) {
	case 0:
		record(task.ID, fmt.Sprintf("task %s has zero parent phases", task.ID))
	case 1:
	default:
		record(task.ID, fmt.Sprintf("task %s has multiple parent phases: %s", task.ID, strings.Join(parents, ", ")))
	}
}

// checkTaskDependencies reports self-dependencies, dependencies on deprecated
// tasks, and dependencies that cross a phase boundary.
func checkTaskDependencies(task model.Entity, graph taskGraph, record func(entity, message string)) {
	parents := graph.parents[task.ID]

	for _, prerequisiteID := range graph.dependencies[task.ID] {
		if prerequisiteID == task.ID {
			record(task.ID, fmt.Sprintf("task %s has self-dependency %s -> %s", task.ID, task.ID, prerequisiteID))
			continue
		}
		prerequisite, ok := graph.byID[prerequisiteID]
		if !ok {
			continue
		}
		if prerequisite.Status == model.EntityStatusDeprecated {
			record(task.ID, fmt.Sprintf("task %s depends on deprecated task %s", task.ID, prerequisiteID))
		}

		prerequisiteParents := graph.parents[prerequisiteID]
		if len(parents) == 1 && len(prerequisiteParents) == 1 && parents[0] != prerequisiteParents[0] {
			record(task.ID, fmt.Sprintf("cross-phase task dependency %s (%s) -> %s (%s)",
				task.ID, parents[0], prerequisiteID, prerequisiteParents[0]))
		}
	}
}

// findTaskCycles reports every task participating in a task_depends_on cycle.
func findTaskCycles(tasks []model.Entity, graph taskGraph, record func(entity, message string)) {
	visited := make(map[string]bool, len(tasks))
	inStack := make(map[string]bool, len(tasks))
	var stack []string

	var visit func(string)
	visit = func(taskID string) {
		visited[taskID] = true
		inStack[taskID] = true
		stack = append(stack, taskID)

		for _, prerequisiteID := range graph.dependencies[taskID] {
			if prerequisiteID == taskID {
				continue
			}
			if _, ok := graph.byID[prerequisiteID]; !ok {
				continue
			}
			if !visited[prerequisiteID] {
				visit(prerequisiteID)
				continue
			}
			if !inStack[prerequisiteID] {
				continue
			}

			members := slices.Clone(stack[slices.Index(stack, prerequisiteID):])
			description := strings.Join(append(slices.Clone(members), prerequisiteID), " -> ")
			for _, member := range members {
				record(member, fmt.Sprintf("task dependency cycle members [%s]: %s",
					strings.Join(members, ", "), description))
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
}

// checkPhaseOrder detects duplicate order values among phases within the same plan.
func checkPhaseOrder(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, err := execEntitiesOfType(ef, model.EntityTypePhase)
	if err != nil {
		return []ValidationIssue{listFailureIssue("phase_order", model.LayerExec, err)}
	}

	ordersByPlan, issues := phaseOrdersByPlan(phases, rf)

	for _, orders := range ordersByPlan {
		idsByOrder := make(map[int][]string)
		for id, order := range orders {
			idsByOrder[order] = append(idsByOrder[order], id)
		}
		for order, ids := range idsByOrder {
			if len(ids) <= 1 {
				continue
			}
			sort.Strings(ids)
			for _, id := range ids {
				issues = append(issues, execIssue("phase_order", SeverityHigh, id,
					fmt.Sprintf("duplicate phase order %d", order)))
			}
		}
	}

	return issues
}

// phaseOrdersByPlan maps each plan to its phases' declared order values. Phases
// with no parent plan or no order in metadata are not ordered and are omitted.
func phaseOrdersByPlan(phases []model.Entity, rf RelationFetcher) (map[string]map[string]int, []ValidationIssue) {
	ordersByPlan := make(map[string]map[string]int)
	var issues []ValidationIssue

	for _, p := range phases {
		rels, issue := fetchRelations(rf, p.ID, "phase_order", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		planID := parentPlanID(p.ID, rels)
		if planID == "" {
			continue
		}

		var meta struct {
			Order *int `json:"order"`
		}
		if err := json.Unmarshal(p.Metadata, &meta); err != nil || meta.Order == nil {
			continue
		}

		if ordersByPlan[planID] == nil {
			ordersByPlan[planID] = make(map[string]int)
		}
		ordersByPlan[planID][p.ID] = *meta.Order
	}

	return ordersByPlan, issues
}

func parentPlanID(phaseID string, rels []model.Relation) string {
	for _, r := range rels {
		if r.Type == model.RelationBelongsTo && r.FromID == phaseID {
			return r.ToID
		}
	}
	return ""
}

// checkSingleActivePlan reports when more than one plan has status=active.
func checkSingleActivePlan(ef EntityFetcher) []ValidationIssue {
	planType := model.EntityTypePlan
	activeStatus := model.EntityStatusActive
	layer := model.LayerExec
	plans, err := ef.List(EntityListFilters{Type: &planType, Status: &activeStatus, Layer: &layer})
	if err != nil {
		return []ValidationIssue{listFailureIssue("single_active_plan", model.LayerExec, err)}
	}

	if len(plans) <= 1 {
		return nil
	}

	issues := make([]ValidationIssue, 0, len(plans))
	for _, p := range plans {
		issues = append(issues, execIssue("single_active_plan", SeverityHigh, p.ID,
			fmt.Sprintf("multiple active plans detected (%d total)", len(plans))))
	}

	return issues
}

// checkOrphanPhases finds phases that have no belongs_to relation to any plan.
func checkOrphanPhases(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, err := execEntitiesOfType(ef, model.EntityTypePhase)
	if err != nil {
		return []ValidationIssue{listFailureIssue("orphan_phases", model.LayerExec, err)}
	}

	var issues []ValidationIssue
	for _, p := range phases {
		rels, issue := fetchRelations(rf, p.ID, "orphan_phases", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		if parentPlanID(p.ID, rels) == "" {
			issues = append(issues, execIssue("orphan_phases", SeverityMedium, p.ID,
				"phase does not belong to any plan"))
		}
	}

	return issues
}

// checkExecCycles detects circular blocks relations between phases using DFS.
func checkExecCycles(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	phases, err := execEntitiesOfType(ef, model.EntityTypePhase)
	if err != nil {
		return []ValidationIssue{listFailureIssue("exec_cycles", model.LayerExec, err)}
	}

	phaseIDs, adj, issues := buildBlocksGraph(phases, rf)
	issues = append(issues, findBlocksCycles(phases, phaseIDs, adj)...)
	return issues
}

// buildBlocksGraph collects the blocks adjacency between phases.
func buildBlocksGraph(phases []model.Entity, rf RelationFetcher) (map[string]bool, map[string][]string, []ValidationIssue) {
	phaseIDs := make(map[string]bool, len(phases))
	adj := make(map[string][]string)
	var issues []ValidationIssue

	for _, p := range phases {
		phaseIDs[p.ID] = true
		rels, issue := fetchRelations(rf, p.ID, "exec_cycles", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		for _, r := range rels {
			if r.Type == model.RelationBlocks && r.FromID == p.ID {
				adj[p.ID] = append(adj[p.ID], r.ToID)
			}
		}
	}

	return phaseIDs, adj, issues
}

// findBlocksCycles reports every phase participating in a blocks cycle.
func findBlocksCycles(phases []model.Entity, phaseIDs map[string]bool, adj map[string][]string) []ValidationIssue {
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
				continue
			}
			if !recStack[next] {
				continue
			}

			cycle := path[slices.Index(path, next):]
			cycleDesc := fmt.Sprintf("%s → %s", formatCyclePath(cycle), next)
			for _, id := range cycle {
				issues = append(issues, execIssue("exec_cycles", SeverityHigh, id,
					fmt.Sprintf("circular blocks dependency detected: %s", cycleDesc)))
			}
			return true
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
	return checkInvalidExecEdgesFor(ValidateOptions{}, rf, ef)
}

func checkInvalidExecEdgesFor(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	subjects, err := resolveValidationSubjects(opts, rf, ef)
	if err != nil {
		return []ValidationIssue{execIssue("invalid_exec_edges", SeverityHigh, "",
			fmt.Sprintf("could not resolve validation subjects: %v", err))}
	}

	entities, issues := execEdgeSubjects(subjects, ef)

	seen := make(map[string]bool)
	for _, e := range entities {
		rels, issue := fetchRelations(rf, e.ID, "invalid_exec_edges", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
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

			issues = append(issues, execEdgeIssues(rel, subjects, ef)...)
		}
	}

	return issues
}

// execEdgeSubjects returns the entities whose exec edges should be checked.
func execEdgeSubjects(subjects validationSubjects, ef EntityFetcher) ([]model.Entity, []ValidationIssue) {
	if !subjects.scoped {
		entities, err := execEntities(ef)
		if err != nil {
			return nil, []ValidationIssue{listFailureIssue("invalid_exec_edges", model.LayerExec, err)}
		}
		return entities, nil
	}

	var entities []model.Entity
	var issues []ValidationIssue
	for id := range subjects.execIDs {
		entity, issue := fetchEntity(ef, id, "invalid_exec_edges", model.LayerExec)
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

// execEdgeIssues reports a relation whose endpoints the exec edge matrix forbids.
func execEdgeIssues(rel model.Relation, subjects validationSubjects, ef EntityFetcher) []ValidationIssue {
	srcEntity, issue := fetchEntity(ef, rel.FromID, "invalid_exec_edges", model.LayerExec)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	tgtEntity, issue := fetchEntity(ef, rel.ToID, "invalid_exec_edges", model.LayerExec)
	if issue != nil {
		return []ValidationIssue{*issue}
	}
	if srcEntity == nil || tgtEntity == nil {
		return nil
	}

	execLayer := model.LayerExec
	if model.IsEdgeAllowed(rel.Type, srcEntity.Type, tgtEntity.Type, &execLayer) {
		return nil
	}

	// Attribute the issue to whichever endpoint is in scope, so a scoped run
	// does not filter out its own finding.
	issueEntity := rel.FromID
	if subjects.scoped && !subjects.execIDs[issueEntity] {
		issueEntity = rel.ToID
	}
	return []ValidationIssue{execIssue("invalid_exec_edges", SeverityHigh, issueEntity,
		fmt.Sprintf("relation %q not allowed from %q to %q", rel.Type, srcEntity.Type, tgtEntity.Type))}
}

// checkOrphanChanges finds CHG entities that have no relation to any non-CHG entity.
func checkOrphanChanges(rf RelationFetcher, ef EntityFetcher) []ValidationIssue {
	changes, err := execEntitiesOfType(ef, model.EntityTypeChange)
	if err != nil {
		return []ValidationIssue{listFailureIssue("orphan_changes", model.LayerExec, err)}
	}

	var issues []ValidationIssue
	for _, chg := range changes {
		rels, issue := fetchRelations(rf, chg.ID, "orphan_changes", model.LayerExec)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}

		linked, issue := hasNonChangeRelation(chg.ID, rels, ef)
		if issue != nil {
			issues = append(issues, *issue)
			continue
		}
		if linked {
			continue
		}

		issues = append(issues, execIssue("orphan_changes", orphanChangeSeverity(chg.Status), chg.ID,
			"change has no relations to other entities"))
	}

	return issues
}

// hasNonChangeRelation reports whether a change is linked to any entity that is
// not itself a change. A dangling reference counts as linked: the target is
// reported by relation validation, not here.
func hasNonChangeRelation(changeID string, rels []model.Relation, ef EntityFetcher) (bool, *ValidationIssue) {
	for _, r := range rels {
		otherID := r.ToID
		if otherID == changeID {
			otherID = r.FromID
		}
		other, issue := fetchEntity(ef, otherID, "orphan_changes", model.LayerExec)
		if issue != nil {
			return false, issue
		}
		if other == nil || other.Type != model.EntityTypeChange {
			return true, nil
		}
	}
	return false, nil
}

// orphanChangeSeverity escalates an orphaned change once it has left draft.
func orphanChangeSeverity(status model.EntityStatus) Severity {
	switch status {
	case model.EntityStatusActive, model.EntityStatusResolved, model.EntityStatusDeprecated:
		return SeverityHigh
	default:
		return SeverityMedium
	}
}

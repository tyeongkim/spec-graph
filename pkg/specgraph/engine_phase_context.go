package specgraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// PhaseContextResult is the complete engine-owned context for executing a phase.
type PhaseContextResult struct {
	Plan           model.Entity        `json:"plan"`
	Phase          model.Entity        `json:"phase"`
	Tasks          []TaskContext       `json:"tasks"`
	Scope          []model.Entity      `json:"scope"`
	Delivery       []string            `json:"delivery"`
	Blockers       map[string][]string `json:"blockers"`
	ReadyTaskIDs   []string            `json:"ready_task_ids"`
	BlockedTaskIDs []string            `json:"blocked_task_ids"`
}

// TaskContext combines a complete task entity with its decoded execution contract and graph mappings.
type TaskContext struct {
	Entity          model.Entity       `json:"entity"`
	Contract        model.TaskContract `json:"contract"`
	PrerequisiteIDs []string           `json:"prerequisite_ids"`
	Covers          []string           `json:"covers"`
	Delivers        []string           `json:"delivers"`
}

type phaseTaskNode struct {
	context    TaskContext
	dependents []string
	indegree   int
}

// PhaseContext returns a deterministic, non-persisted execution context for phaseID.
func (e *Engine) PhaseContext(phaseID string) (PhaseContextResult, error) {
	return readLocked(e, func() (PhaseContextResult, error) {
		return e.phaseContextLocked(phaseID)
	})
}

func (e *Engine) phaseContextLocked(phaseID string) (PhaseContextResult, error) {
	phase, err := e.phaseContextEntity(phaseID, model.EntityTypePhase)
	if err != nil {
		return PhaseContextResult{}, err
	}

	plan, err := e.phaseParentPlan(phase)
	if err != nil {
		return PhaseContextResult{}, err
	}

	effective, err := graph.EffectivePhaseScope(phaseID, &engineRelationFetcher{idx: e.idx})
	if err != nil {
		return PhaseContextResult{}, newError(CodeRuntime, fmt.Sprintf("derive phase scope for %q", phaseID), err)
	}
	tasks, err := e.phaseTaskContexts(effective.TaskIDs)
	if err != nil {
		return PhaseContextResult{}, err
	}
	scope, err := e.phaseCoveredEntities(effective.Covered)
	if err != nil {
		return PhaseContextResult{}, err
	}

	ready, blocked, blockers := classifyPhaseTasks(tasks)
	return PhaseContextResult{
		Plan:           plan,
		Phase:          phase,
		Tasks:          tasks,
		Scope:          scope,
		Delivery:       nonNilStrings(effective.Delivered),
		Blockers:       blockers,
		ReadyTaskIDs:   ready,
		BlockedTaskIDs: blocked,
	}, nil
}

func (e *Engine) phaseContextEntity(id string, expected model.EntityType) (model.Entity, error) {
	if id == "" {
		return model.Entity{}, newError(CodeInvalidInput, "phase id is required", nil)
	}
	record, err := e.idx.GetEntity(id)
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("lookup entity %q", id), err)
	}
	if record == nil {
		return model.Entity{}, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
	}
	entity := engineEntityFromRecord(record)
	if entity.Type != expected {
		return model.Entity{}, newError(CodeInvalidInput, fmt.Sprintf("entity %q is type %q, not %s", id, entity.Type, expected), nil)
	}
	return entity, nil
}

func (e *Engine) phaseParentPlan(phase model.Entity) (model.Entity, error) {
	relations, err := e.idx.GetRelationsByEntity(phase.ID)
	if err != nil {
		return model.Entity{}, newError(CodeRuntime, fmt.Sprintf("lookup phase relations for %q", phase.ID), err)
	}
	planIDs := make([]string, 0)
	for _, relation := range relations {
		if relation.FromID == phase.ID && relation.Type == string(model.RelationBelongsTo) {
			planIDs = append(planIDs, relation.ToID)
		}
	}
	sort.Strings(planIDs)
	if len(planIDs) != 1 {
		return model.Entity{}, newError(
			CodeInvalidState,
			fmt.Sprintf("phase_parent: phase %s requires exactly one parent plan, found %v", phase.ID, planIDs),
			nil,
		)
	}
	plan, err := e.phaseContextEntity(planIDs[0], model.EntityTypePlan)
	if err != nil {
		return model.Entity{}, newError(CodeInvalidState, fmt.Sprintf("phase_parent: phase %s parent %s is invalid", phase.ID, planIDs[0]), err)
	}
	return plan, nil
}

func (e *Engine) phaseTaskContexts(taskIDs []string) ([]TaskContext, error) {
	nodes := make(map[string]*phaseTaskNode, len(taskIDs))
	for _, taskID := range taskIDs {
		task, err := e.phaseContextEntity(taskID, model.EntityTypeTask)
		if err != nil {
			return nil, newError(CodeInvalidState, fmt.Sprintf("task_graph: phase task %s is invalid", taskID), err)
		}
		contract, err := model.DecodeTaskContract(task.Metadata, task.Status)
		if err != nil {
			return nil, newError(CodeInvalidState, fmt.Sprintf("task_contract: task %s: %v", taskID, err), err)
		}
		relations, err := e.idx.GetRelationsByEntity(taskID)
		if err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("lookup task relations for %q", taskID), err)
		}
		context := TaskContext{Entity: task, Contract: contract, PrerequisiteIDs: []string{}, Covers: []string{}, Delivers: []string{}}
		for _, relation := range relations {
			if relation.FromID != taskID {
				continue
			}
			switch relation.Type {
			case string(model.RelationTaskDependsOn):
				context.PrerequisiteIDs = append(context.PrerequisiteIDs, relation.ToID)
			case string(model.RelationCovers):
				context.Covers = append(context.Covers, relation.ToID)
			case string(model.RelationDelivers):
				context.Delivers = append(context.Delivers, relation.ToID)
			}
		}
		sort.Strings(context.PrerequisiteIDs)
		sort.Strings(context.Covers)
		sort.Strings(context.Delivers)
		nodes[taskID] = &phaseTaskNode{context: context, indegree: len(context.PrerequisiteIDs)}
	}

	for taskID, node := range nodes {
		for _, prerequisiteID := range node.context.PrerequisiteIDs {
			prerequisite, exists := nodes[prerequisiteID]
			if !exists {
				return nil, newError(CodeInvalidState, fmt.Sprintf("task_graph: task %s prerequisite %s is outside the phase", taskID, prerequisiteID), nil)
			}
			prerequisite.dependents = append(prerequisite.dependents, taskID)
		}
	}
	for _, node := range nodes {
		sort.Strings(node.dependents)
	}
	return sortPhaseTaskContexts(nodes)
}

func sortPhaseTaskContexts(nodes map[string]*phaseTaskNode) ([]TaskContext, error) {
	queue := make([]string, 0, len(nodes))
	for taskID, node := range nodes {
		if node.indegree == 0 {
			queue = append(queue, taskID)
		}
	}
	sortPhaseTaskQueue(queue, nodes)

	ordered := make([]TaskContext, 0, len(nodes))
	for len(queue) > 0 {
		taskID := queue[0]
		queue = queue[1:]
		node := nodes[taskID]
		ordered = append(ordered, node.context)
		for _, dependentID := range node.dependents {
			dependent := nodes[dependentID]
			dependent.indegree--
			if dependent.indegree == 0 {
				queue = append(queue, dependentID)
			}
		}
		sortPhaseTaskQueue(queue, nodes)
	}
	if len(ordered) != len(nodes) {
		members := taskCycleMembers(nodes)
		return nil, newError(CodeInvalidState, fmt.Sprintf("task_graph: task dependency cycle members [%s]", strings.Join(members, ", ")), nil)
	}
	return ordered, nil
}

func sortPhaseTaskQueue(queue []string, nodes map[string]*phaseTaskNode) {
	sort.Slice(queue, func(left, right int) bool {
		leftOrder := nodes[queue[left]].context.Contract.Order
		rightOrder := nodes[queue[right]].context.Contract.Order
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return queue[left] < queue[right]
	})
}

func taskCycleMembers(nodes map[string]*phaseTaskNode) []string {
	visited := make(map[string]bool, len(nodes))
	inStack := make(map[string]bool, len(nodes))
	memberSet := make(map[string]bool)
	stack := make([]string, 0, len(nodes))
	var visit func(string)
	visit = func(taskID string) {
		visited[taskID] = true
		inStack[taskID] = true
		stack = append(stack, taskID)
		for _, prerequisiteID := range nodes[taskID].context.PrerequisiteIDs {
			if !visited[prerequisiteID] {
				visit(prerequisiteID)
				continue
			}
			if inStack[prerequisiteID] {
				start := 0
				for stack[start] != prerequisiteID {
					start++
				}
				for _, member := range stack[start:] {
					memberSet[member] = true
				}
			}
		}
		stack = stack[:len(stack)-1]
		inStack[taskID] = false
	}
	taskIDs := make([]string, 0, len(nodes))
	for taskID := range nodes {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		if !visited[taskID] {
			visit(taskID)
		}
	}
	members := make([]string, 0, len(memberSet))
	for member := range memberSet {
		members = append(members, member)
	}
	sort.Strings(members)
	return members
}

func (e *Engine) phaseCoveredEntities(ids []string) ([]model.Entity, error) {
	entities := make([]model.Entity, 0, len(ids))
	for _, id := range ids {
		record, err := e.idx.GetEntity(id)
		if err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("lookup covered entity %q", id), err)
		}
		if record == nil {
			return nil, newError(CodeInvalidState, fmt.Sprintf("phase_scope: covered entity %s does not exist", id), nil)
		}
		entity := engineEntityFromRecord(record)
		if entity.Layer != model.LayerArch {
			return nil, newError(CodeInvalidState, fmt.Sprintf("phase_scope: covered entity %s is not architecture", id), nil)
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func classifyPhaseTasks(tasks []TaskContext) ([]string, []string, map[string][]string) {
	statuses := make(map[string]model.EntityStatus, len(tasks))
	for _, task := range tasks {
		statuses[task.Entity.ID] = task.Entity.Status
	}
	ready := make([]string, 0)
	blocked := make([]string, 0)
	blockers := make(map[string][]string)
	for _, task := range tasks {
		if task.Entity.Status != model.EntityStatusDraft && task.Entity.Status != model.EntityStatusActive {
			continue
		}
		unresolved := make([]string, 0)
		for _, prerequisiteID := range task.PrerequisiteIDs {
			if statuses[prerequisiteID] != model.EntityStatusResolved {
				unresolved = append(unresolved, prerequisiteID)
			}
		}
		if len(unresolved) == 0 {
			ready = append(ready, task.Entity.ID)
			continue
		}
		blocked = append(blocked, task.Entity.ID)
		blockers[task.Entity.ID] = unresolved
	}
	return ready, blocked, blockers
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

func evaluateLifecycle(target Target, rf validate.RelationFetcher, ef validate.EntityFetcher) ([]validate.ValidationIssue, []validate.ValidationIssue) {
	if isTerminal(target.FromStatus) && target.ToStatus != target.FromStatus {
		return []validate.ValidationIssue{gateIssue("lifecycle", target.EntityID, "terminal entities cannot transition to another status", model.LayerExec)}, nil
	}

	switch target.EntityType {
	case model.EntityTypeTask:
		return evaluateTask(target, rf, ef)
	case model.EntityTypePhase:
		return evaluatePhase(target, rf, ef)
	case model.EntityTypePlan:
		return evaluatePlan(target, rf, ef)
	default:
		return nil, nil
	}
}

func evaluateTask(target Target, rf validate.RelationFetcher, ef validate.EntityFetcher) ([]validate.ValidationIssue, []validate.ValidationIssue) {
	contract, err := model.DecodeTaskContract(target.Candidate.Metadata, target.ToStatus)
	if err != nil {
		return []validate.ValidationIssue{gateIssue("task_contract", target.EntityID, err.Error(), model.LayerExec)}, nil
	}
	relations, err := rf.GetByEntity(target.EntityID)
	if err != nil {
		return []validate.ValidationIssue{gateIssue("task_graph", target.EntityID, err.Error(), model.LayerExec)}, nil
	}
	parents := relationTargets(relations, target.EntityID, model.RelationBelongsTo)
	prerequisites := relationTargets(relations, target.EntityID, model.RelationTaskDependsOn)

	var activation []validate.ValidationIssue
	if len(parents) != 1 {
		activation = append(activation, gateIssue("task_parent", target.EntityID, fmt.Sprintf("task requires exactly one parent phase, found %d", len(parents)), model.LayerExec))
	} else if parent, getErr := ef.Get(parents[0]); getErr != nil || parent.Type != model.EntityTypePhase {
		activation = append(activation, gateIssue("task_parent", target.EntityID, "task parent must be a phase", model.LayerExec))
	} else if parent.Status != model.EntityStatusActive {
		return activation, []validate.ValidationIssue{gateIssue("task_parent_status", target.EntityID, "task parent phase must be active", model.LayerExec)}
	}
	if target.ToStatus == model.EntityStatusActive {
		return activation, unresolvedEntities("task_prerequisites", target.EntityID, prerequisites, ef)
	}

	var completion []validate.ValidationIssue
	completion = append(completion, unresolvedEntities("task_prerequisites", target.EntityID, prerequisites, ef)...)
	for index, qa := range contract.QA {
		if err := validateEvidencePath(target.RepoRoot, qa.Evidence); err != nil {
			completion = append(completion, gateIssue("task_evidence", target.EntityID, fmt.Sprintf("qa[%d] evidence: %v", index, err), model.LayerExec))
		}
	}
	completion = append(completion, missingTaskDeliveries(target.EntityID, relations, ef)...)
	return nil, completion
}

func evaluatePhase(target Target, rf validate.RelationFetcher, ef validate.EntityFetcher) ([]validate.ValidationIssue, []validate.ValidationIssue) {
	relations, err := rf.GetByEntity(target.EntityID)
	if err != nil {
		return []validate.ValidationIssue{gateIssue("phase_relations", target.EntityID, err.Error(), model.LayerExec)}, nil
	}
	if target.ToStatus == model.EntityStatusActive {
		var issues []validate.ValidationIssue
		parents := relationTargets(relations, target.EntityID, model.RelationBelongsTo)
		if len(parents) != 1 {
			issues = append(issues, gateIssue("phase_parent", target.EntityID, fmt.Sprintf("phase requires exactly one parent plan, found %d", len(parents)), model.LayerExec))
		} else if parent, getErr := ef.Get(parents[0]); getErr != nil || parent.Type != model.EntityTypePlan {
			issues = append(issues, gateIssue("phase_parent", target.EntityID, "phase parent must be a plan", model.LayerExec))
		} else if parent.Status != model.EntityStatusActive {
			return issues, []validate.ValidationIssue{gateIssue("phase_parent_status", target.EntityID, "phase parent plan must be active", model.LayerExec)}
		}
		return issues, unresolvedEntities("phase_dependencies", target.EntityID, incomingSources(relations, target.EntityID, model.RelationPrecedes, model.RelationBlocks), ef)
	}

	children := incomingSources(relations, target.EntityID, model.RelationBelongsTo)
	return nil, unresolvedChildren("phase_tasks", target.EntityID, children, model.EntityTypeTask, ef)
}

func evaluatePlan(target Target, rf validate.RelationFetcher, ef validate.EntityFetcher) ([]validate.ValidationIssue, []validate.ValidationIssue) {
	relations, err := rf.GetByEntity(target.EntityID)
	if err != nil {
		return []validate.ValidationIssue{gateIssue("plan_relations", target.EntityID, err.Error(), model.LayerExec)}, nil
	}
	children := incomingSources(relations, target.EntityID, model.RelationBelongsTo)
	return nil, unresolvedChildren("plan_phases", target.EntityID, children, model.EntityTypePhase, ef)
}

func unresolvedChildren(check, owner string, ids []string, entityType model.EntityType, ef validate.EntityFetcher) []validate.ValidationIssue {
	var issues []validate.ValidationIssue
	for _, id := range ids {
		entity, err := ef.Get(id)
		if err != nil || entity.Type != entityType || entity.Status == model.EntityStatusDeprecated || entity.Status == model.EntityStatusDeleted {
			continue
		}
		if entity.Status != model.EntityStatusResolved {
			issues = append(issues, gateIssue(check, owner, fmt.Sprintf("child %s %s is not resolved", entityType, id), model.LayerExec))
		}
	}
	return issues
}

func unresolvedEntities(check, owner string, ids []string, ef validate.EntityFetcher) []validate.ValidationIssue {
	var issues []validate.ValidationIssue
	for _, id := range ids {
		entity, err := ef.Get(id)
		if err != nil || entity.Status != model.EntityStatusResolved {
			issues = append(issues, gateIssue(check, owner, fmt.Sprintf("prerequisite %s is not resolved", id), model.LayerExec))
		}
	}
	return issues
}

func missingTaskDeliveries(taskID string, relations []model.Relation, ef validate.EntityFetcher) []validate.ValidationIssue {
	covered := make(map[string]bool)
	delivered := make(map[string]bool)
	for _, relation := range relations {
		if relation.FromID != taskID {
			continue
		}
		switch relation.Type {
		case model.RelationCovers:
			covered[relation.ToID] = true
		case model.RelationDelivers:
			delivered[relation.ToID] = true
		}
	}
	var issues []validate.ValidationIssue
	mapping := model.LayerMapping
	for id := range covered {
		entity, err := ef.Get(id)
		if err != nil || !model.IsEdgeAllowed(model.RelationDelivers, model.EntityTypeTask, entity.Type, &mapping) || delivered[id] {
			continue
		}
		issues = append(issues, gateIssue("task_delivery", taskID, fmt.Sprintf("covered deliverable %s is not delivered", id), model.LayerMapping))
	}
	return issues
}

func validateEvidencePath(repoRoot, evidence string) error {
	clean := filepath.Clean(strings.TrimSpace(evidence))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("must be a non-empty repository-relative path")
	}
	path := filepath.Join(repoRoot, clean)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", clean, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", clean)
	}
	return nil
}

func relationTargets(relations []model.Relation, from string, relationType model.RelationType) []string {
	var ids []string
	for _, relation := range relations {
		if relation.FromID == from && relation.Type == relationType {
			ids = append(ids, relation.ToID)
		}
	}
	return ids
}

func incomingSources(relations []model.Relation, to string, relationTypes ...model.RelationType) []string {
	allowed := make(map[model.RelationType]bool, len(relationTypes))
	for _, relationType := range relationTypes {
		allowed[relationType] = true
	}
	var ids []string
	for _, relation := range relations {
		if relation.ToID == to && allowed[relation.Type] {
			ids = append(ids, relation.FromID)
		}
	}
	return ids
}

func gateIssue(check, entity, message string, layer model.Layer) validate.ValidationIssue {
	return validate.ValidationIssue{Check: check, Severity: validate.SeverityHigh, Entity: entity, Message: message, Layer: layer}
}

func isTerminal(status model.EntityStatus) bool {
	return status == model.EntityStatusResolved || status == model.EntityStatusDeprecated || status == model.EntityStatusDeleted
}

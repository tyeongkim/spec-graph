package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// PhaseNextRequest describes the inputs for finding the next eligible phase.
type PhaseNextRequest struct {
	// Activate, when true, transitions the next phase from draft to active.
	Activate bool
}

// PhaseNextResult holds the outcome of a PhaseNext call.
type PhaseNextResult struct {
	Phase     PhaseNextDetail `json:"phase"`
	Scope     PhaseNextScope  `json:"scope"`
	Activated bool            `json:"activated"`
}

// PhaseNextDetail describes the next eligible phase.
type PhaseNextDetail struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Status               string          `json:"status"`
	Goal                 string          `json:"goal"`
	Order                float64         `json:"order"`
	PredecessorsResolved bool            `json:"predecessors_resolved"`
	Metadata             json.RawMessage `json:"metadata"`
}

// PhaseNextScope describes delivery progress within a phase.
type PhaseNextScope struct {
	Total     int      `json:"total"`
	Delivered int      `json:"delivered"`
	Remaining []string `json:"remaining"`
}

type enginePhaseInfo struct {
	record index.EntityRecord
	order  float64
	status model.EntityStatus
}

// PhaseNext finds the next eligible phase in the active plan. A phase is
// eligible when it is neither resolved nor deprecated and all of its
// predecessor phases are resolved; among eligible phases the one with the
// lowest order is selected. When req.Activate is true and the selected phase is
// in draft, it is transitioned to active. The provided context is accepted for
// forward compatibility and is not yet observed.
func (e *Engine) PhaseNext(ctx context.Context, req PhaseNextRequest) (PhaseNextResult, error) {
	_ = ctx

	return writeLocked(e, func() (PhaseNextResult, error) {
		return e.phaseNextLocked(req)
	})
}

func (e *Engine) phaseNextLocked(req PhaseNextRequest) (PhaseNextResult, error) {
	activePlanID, err := e.findActivePlan()
	if err != nil {
		return PhaseNextResult{}, err
	}

	planPhaseIDs := e.collectPlanPhases(activePlanID)
	if len(planPhaseIDs) == 0 {
		return PhaseNextResult{}, newError(
			CodeInvalidInput,
			fmt.Sprintf("active plan %s has no phases", activePlanID),
			nil,
		)
	}

	phases := e.buildPhaseInfoMap(planPhaseIDs)
	predecessors := e.buildPredecessorMap(planPhaseIDs)
	nextID, nextPhase := selectNextEnginePhase(phases, predecessors)

	if nextID == "" {
		return PhaseNextResult{}, newError(
			CodeInvalidInput,
			"no eligible next phase found; all phases are resolved or have unresolved predecessors",
			nil,
		)
	}

	scope, err := e.computePhaseScope(nextID)
	if err != nil {
		return PhaseNextResult{}, err
	}

	activated := false
	finalStatus := string(nextPhase.status)
	if req.Activate && nextPhase.status == model.EntityStatusDraft {
		if err := e.activatePhase(nextID); err != nil {
			return PhaseNextResult{}, err
		}
		activated = true
		finalStatus = string(model.EntityStatusActive)
	}

	goal := extractPhaseGoal(nextPhase.record.Metadata)

	return PhaseNextResult{
		Phase: PhaseNextDetail{
			ID:                   nextID,
			Title:                nextPhase.record.Title,
			Status:               finalStatus,
			Goal:                 goal,
			Order:                nextPhase.order,
			PredecessorsResolved: true,
			Metadata:             json.RawMessage(nextPhase.record.Metadata),
		},
		Scope:     scope,
		Activated: activated,
	}, nil
}

func (e *Engine) findActivePlan() (string, error) {
	planRecs, err := e.idx.ListEntities(index.EntityFilters{
		Type:   string(model.EntityTypePlan),
		Status: string(model.EntityStatusActive),
	})
	if err != nil {
		return "", newError(CodeRuntime, "list plans", err)
	}
	if len(planRecs) == 0 {
		return "", newError(
			CodeNotFound,
			"no active plan found; create and activate a plan first",
			nil,
		)
	}

	return planRecs[0].ID, nil
}

func (e *Engine) collectPlanPhases(planID string) map[string]bool {
	allPhaseRecs, err := e.idx.ListEntities(index.EntityFilters{
		Type: string(model.EntityTypePhase),
	})
	if err != nil {
		return nil
	}

	result := make(map[string]bool)
	for _, rec := range allPhaseRecs {
		rels, relErr := e.idx.GetRelationsByEntity(rec.ID)
		if relErr != nil {
			continue
		}
		for _, rel := range rels {
			if rel.FromID == rec.ID && rel.Type == string(model.RelationBelongsTo) && rel.ToID == planID {
				result[rec.ID] = true
				break
			}
		}
	}

	return result
}

func (e *Engine) buildPhaseInfoMap(planPhaseIDs map[string]bool) map[string]*enginePhaseInfo {
	allPhaseRecs, _ := e.idx.ListEntities(index.EntityFilters{
		Type: string(model.EntityTypePhase),
	})

	phases := make(map[string]*enginePhaseInfo)
	for _, rec := range allPhaseRecs {
		if !planPhaseIDs[rec.ID] {
			continue
		}
		pi := &enginePhaseInfo{
			record: rec,
			status: model.EntityStatus(rec.Status),
		}
		if rec.Metadata != "" {
			var meta map[string]any
			if err := json.Unmarshal([]byte(rec.Metadata), &meta); err == nil {
				if o, ok := meta["order"]; ok {
					if v, ok := o.(float64); ok {
						pi.order = v
					}
				}
			}
		}
		phases[rec.ID] = pi
	}

	return phases
}

func (e *Engine) buildPredecessorMap(planPhaseIDs map[string]bool) map[string][]string {
	predecessors := make(map[string][]string)
	for phaseID := range planPhaseIDs {
		rels, relErr := e.idx.GetRelationsByEntity(phaseID)
		if relErr != nil {
			continue
		}
		for _, rel := range rels {
			if rel.Type == string(model.RelationPrecedes) && rel.ToID == phaseID {
				predecessors[phaseID] = append(predecessors[phaseID], rel.FromID)
			}
		}
	}

	return predecessors
}

func selectNextEnginePhase(phases map[string]*enginePhaseInfo, predecessors map[string][]string) (string, *enginePhaseInfo) {
	type candidate struct {
		id    string
		order float64
	}
	var candidates []candidate

	for phaseID, pi := range phases {
		if pi.status == model.EntityStatusResolved || pi.status == model.EntityStatusDeprecated {
			continue
		}

		allResolved := true
		for _, predID := range predecessors[phaseID] {
			pred, ok := phases[predID]
			if !ok || pred.status != model.EntityStatusResolved {
				allResolved = false
				break
			}
		}

		if allResolved {
			candidates = append(candidates, candidate{id: phaseID, order: pi.order})
		}
	}

	if len(candidates) == 0 {
		return "", nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].order < candidates[j].order
	})

	return candidates[0].id, phases[candidates[0].id]
}

func (e *Engine) computePhaseScope(phaseID string) (PhaseNextScope, error) {
	scope, err := graph.EffectivePhaseScope(phaseID, &engineRelationFetcher{idx: e.idx})
	if err != nil {
		return PhaseNextScope{}, newError(CodeRuntime, "derive phase scope", err)
	}
	deliveredSet := make(map[string]bool, len(scope.Delivered))
	for _, id := range scope.Delivered {
		deliveredSet[id] = true
	}

	var remaining []string
	for _, id := range scope.Covered {
		if !deliveredSet[id] {
			remaining = append(remaining, id)
		}
	}

	return PhaseNextScope{
		Total:     len(scope.Covered),
		Delivered: len(deliveredSet),
		Remaining: remaining,
	}, nil
}

func (e *Engine) activatePhase(phaseID string) error {
	ef, err := e.store.ReadEntity(phaseID, model.EntityTypePhase)
	if err != nil {
		return newError(CodeRuntime, fmt.Sprintf("read phase entity %q", phaseID), err)
	}
	ef.Status = model.EntityStatusActive
	ef.UpdatedAt = time.Now()
	if err := e.store.WriteEntity(ef); err != nil {
		return newError(CodeRuntime, fmt.Sprintf("write phase entity %q", phaseID), err)
	}

	if _, err := e.syncer.EnsureFresh(); err != nil {
		return newError(CodeRuntime, "sync index after activate phase", err)
	}

	return nil
}

func extractPhaseGoal(metadata string) string {
	if metadata == "" {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		return ""
	}
	if g, ok := meta["goal"].(string); ok {
		return g
	}

	return ""
}

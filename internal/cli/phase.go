package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
)

var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Phase lifecycle commands",
}

type PhaseNextResponse struct {
	Phase     PhaseNextDetail `json:"phase"`
	Scope     PhaseNextScope  `json:"scope"`
	Activated bool            `json:"activated"`
}

type PhaseNextDetail struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Status               string          `json:"status"`
	Goal                 string          `json:"goal"`
	Order                float64         `json:"order"`
	PredecessorsResolved bool            `json:"predecessors_resolved"`
	Metadata             json.RawMessage `json:"metadata"`
}

type PhaseNextScope struct {
	Total     int      `json:"total"`
	Delivered int      `json:"delivered"`
	Remaining []string `json:"remaining"`
}

var phaseNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find and optionally activate the next eligible phase in the active plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		activate, _ := cmd.Flags().GetBool("activate")

		activePlanID, err := findActivePlan()
		if err != nil {
			handleError(cmd, err)
		}

		planPhaseIDs := collectPlanPhases(activePlanID)
		if len(planPhaseIDs) == 0 {
			handleError(cmd, &model.ErrInvalidInput{
				Message: fmt.Sprintf("active plan %s has no phases", activePlanID),
			})
		}

		phases := buildPhaseInfoMap(planPhaseIDs)
		predecessors := buildPredecessorMap(planPhaseIDs)
		nextID, nextPhase := selectNextPhase(phases, predecessors)

		if nextID == "" {
			handleError(cmd, &model.ErrInvalidInput{
				Message: "no eligible next phase found; all phases are resolved or have unresolved predecessors",
			})
		}

		scope := computePhaseScope(nextID)

		activated := false
		finalStatus := string(nextPhase.status)
		if activate && nextPhase.status == model.EntityStatusDraft {
			if err := activatePhase(cmd, nextID); err != nil {
				handleError(cmd, err)
			}
			activated = true
			finalStatus = string(model.EntityStatusActive)
		}

		goal := extractGoal(nextPhase.record.Metadata)

		response := PhaseNextResponse{
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
		}

		writeJSON(cmd, response)
		return nil
	},
}

func findActivePlan() (string, error) {
	planRecs, err := queryIndex.ListEntities(index.EntityFilters{
		Type:   string(model.EntityTypePlan),
		Status: string(model.EntityStatusActive),
	})
	if err != nil {
		return "", fmt.Errorf("list plans: %w", err)
	}
	if len(planRecs) == 0 {
		return "", &model.ErrInvalidInput{
			Message: "no active plan found; create and activate a plan first",
		}
	}

	return planRecs[0].ID, nil
}

func collectPlanPhases(planID string) map[string]bool {
	allPhaseRecs, err := queryIndex.ListEntities(index.EntityFilters{
		Type: string(model.EntityTypePhase),
	})
	if err != nil {
		return nil
	}

	result := make(map[string]bool)
	for _, rec := range allPhaseRecs {
		rels, relErr := queryIndex.GetRelationsByEntity(rec.ID)
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

type phaseInfo struct {
	record index.EntityRecord
	order  float64
	status model.EntityStatus
}

func buildPhaseInfoMap(planPhaseIDs map[string]bool) map[string]*phaseInfo {
	allPhaseRecs, _ := queryIndex.ListEntities(index.EntityFilters{
		Type: string(model.EntityTypePhase),
	})

	phases := make(map[string]*phaseInfo)
	for _, rec := range allPhaseRecs {
		if !planPhaseIDs[rec.ID] {
			continue
		}
		pi := &phaseInfo{
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

func buildPredecessorMap(planPhaseIDs map[string]bool) map[string][]string {
	predecessors := make(map[string][]string)
	for phaseID := range planPhaseIDs {
		rels, relErr := queryIndex.GetRelationsByEntity(phaseID)
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

func selectNextPhase(phases map[string]*phaseInfo, predecessors map[string][]string) (string, *phaseInfo) {
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

func computePhaseScope(phaseID string) PhaseNextScope {
	rf := &indexRelationFetcher{idx: queryIndex}
	rels, _ := rf.GetByEntity(phaseID)

	var coveredIDs []string
	deliveredSet := make(map[string]bool)
	for _, rel := range rels {
		if rel.FromID == phaseID && rel.Type == model.RelationCovers {
			coveredIDs = append(coveredIDs, rel.ToID)
		}
		if rel.FromID == phaseID && rel.Type == model.RelationDelivers {
			deliveredSet[rel.ToID] = true
		}
	}

	var remaining []string
	for _, id := range coveredIDs {
		if !deliveredSet[id] {
			remaining = append(remaining, id)
		}
	}

	return PhaseNextScope{
		Total:     len(coveredIDs),
		Delivered: len(deliveredSet),
		Remaining: remaining,
	}
}

func activatePhase(cmd *cobra.Command, phaseID string) error {
	ef, err := tomlStore.ReadEntity(phaseID, model.EntityTypePhase)
	if err != nil {
		return fmt.Errorf("read phase entity: %w", err)
	}
	ef.Status = model.EntityStatusActive
	ef.UpdatedAt = time.Now()
	if err := tomlStore.WriteEntity(ef); err != nil {
		return fmt.Errorf("write phase entity: %w", err)
	}

	return nil
}

func extractGoal(metadata string) string {
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

func phaseEntityScope(phaseID string, rf *indexRelationFetcher) (map[string]bool, error) {
	rels, err := rf.GetByEntity(phaseID)
	if err != nil {
		return nil, err
	}
	scope := make(map[string]bool)
	for _, r := range rels {
		if r.FromID == phaseID && (r.Type == model.RelationCovers || r.Type == model.RelationDelivers) {
			scope[r.ToID] = true
		}
	}

	return scope, nil
}

func init() {
	phaseNextCmd.Flags().Bool("activate", false, "automatically transition the phase from draft to active")
	phaseCmd.AddCommand(phaseNextCmd)
}

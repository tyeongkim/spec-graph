package mcp

import (
	"context"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type planStatusResult struct {
	ActivePlans  []model.Entity `json:"active_plans"`
	ActivePhases []model.Entity `json:"active_phases"`
}

func planStatus(ctx context.Context, engine *specgraph.Engine) (planStatusResult, error) {
	var result planStatusResult
	err := engine.Read(ctx, func(snapshot *specgraph.Snapshot) error {
		plans, _, err := snapshot.ListEntities(specgraph.ListEntitiesRequest{
			Type:   string(model.EntityTypePlan),
			Status: string(model.EntityStatusActive),
		})
		if err != nil {
			return err
		}

		phases, _, err := snapshot.ListEntities(specgraph.ListEntitiesRequest{
			Type:   string(model.EntityTypePhase),
			Status: string(model.EntityStatusActive),
		})
		if err != nil {
			return err
		}

		result = planStatusResult{ActivePlans: plans, ActivePhases: phases}
		return nil
	})
	if err != nil {
		return planStatusResult{}, err
	}
	return result, nil
}

type phaseBriefResult struct {
	Context specgraph.PhaseContextResult `json:"context"`
	Issues  []validate.ValidationIssue   `json:"issues"`
	Clean   bool                         `json:"clean"`
}

func phaseBrief(ctx context.Context, engine *specgraph.Engine, phaseID string) (phaseBriefResult, error) {
	var result phaseBriefResult
	err := engine.Read(ctx, func(snapshot *specgraph.Snapshot) error {
		phaseContext, err := snapshot.PhaseContext(phaseID)
		if err != nil {
			return err
		}

		issues, err := collectIssues(snapshot, phaseScopedChecks(phaseID))
		if err != nil {
			return err
		}

		result = phaseBriefResult{
			Context: phaseContext,
			Issues:  issues,
			Clean:   len(issues) == 0,
		}
		return nil
	})
	if err != nil {
		return phaseBriefResult{}, err
	}
	return result, nil
}

type phaseGateResult struct {
	Issues   []validate.ValidationIssue `json:"issues"`
	Blockers map[string][]string        `json:"blockers"`
	Passed   bool                       `json:"passed"`
}

// phaseGate runs every graph-level check that governs phase resolution. It
// reports findings without changing state: resolving a phase additionally
// requires the test, build, and per-entity code verification that only the
// caller can perform, so the transition stays an explicit separate call.
func phaseGate(ctx context.Context, engine *specgraph.Engine, phaseID string) (phaseGateResult, error) {
	var result phaseGateResult
	err := engine.Read(ctx, func(snapshot *specgraph.Snapshot) error {
		phaseContext, err := snapshot.PhaseContext(phaseID)
		if err != nil {
			return err
		}

		checks := append(phaseScopedChecks(phaseID),
			specgraph.ValidateRequest{
				Layer:  string(model.LayerMapping),
				Phase:  phaseID,
				Checks: []string{"phase_satisfaction"},
			},
			specgraph.ValidateRequest{
				Layer:  string(model.LayerArch),
				Checks: []string{"coverage", "unresolved"},
			},
		)

		issues, err := collectIssues(snapshot, checks)
		if err != nil {
			return err
		}

		result = phaseGateResult{
			Issues:   issues,
			Blockers: phaseContext.Blockers,
			Passed:   len(issues) == 0 && len(phaseContext.Blockers) == 0,
		}
		return nil
	})
	if err != nil {
		return phaseGateResult{}, err
	}
	return result, nil
}

type changeImpactResult struct {
	Impact    *graph.ImpactResult              `json:"impact"`
	Neighbors map[string]*graph.NeighborResult `json:"neighbors"`
}

func changeImpact(ctx context.Context, engine *specgraph.Engine, req specgraph.ImpactRequest) (changeImpactResult, error) {
	var result changeImpactResult
	err := engine.Read(ctx, func(snapshot *specgraph.Snapshot) error {
		impact, err := snapshot.Impact(req)
		if err != nil {
			return err
		}

		neighbors := make(map[string]*graph.NeighborResult, len(req.Sources))
		for _, source := range req.Sources {
			neighbor, err := snapshot.QueryNeighbors(specgraph.QueryNeighborsRequest{
				EntityID: source,
				Depth:    1,
			})
			if err != nil {
				return err
			}
			neighbors[source] = neighbor
		}

		result = changeImpactResult{Impact: impact, Neighbors: neighbors}
		return nil
	})
	if err != nil {
		return changeImpactResult{}, err
	}
	return result, nil
}

func phaseScopedChecks(phaseID string) []specgraph.ValidateRequest {
	return []specgraph.ValidateRequest{
		{Layer: string(model.LayerMapping), Phase: phaseID},
		{Layer: string(model.LayerExec), Phase: phaseID, Checks: []string{"task_graph"}},
	}
}

func collectIssues(snapshot *specgraph.Snapshot, requests []specgraph.ValidateRequest) ([]validate.ValidationIssue, error) {
	issues := []validate.ValidationIssue{}
	for _, request := range requests {
		result, err := snapshot.Validate(request)
		if err != nil {
			return nil, err
		}
		issues = append(issues, result.Issues...)
	}
	return issues, nil
}

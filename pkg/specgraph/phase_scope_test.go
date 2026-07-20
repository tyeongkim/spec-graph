package specgraph_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestTaskDerivedScopeConsumersAgree(t *testing.T) {
	engine := openTestEngine(t)
	ctx := context.Background()
	createScopeEntity(t, engine, "plan", "PLN-001")
	createScopeEntity(t, engine, "phase", "PHS-001")
	createScopeEntity(t, engine, "requirement", "REQ-001")
	createScopeEntity(t, engine, "interface", "API-001")
	createTask(t, engine, "TSK-001")
	createTask(t, engine, "TSK-002")
	addScopeRelation(t, engine, "PHS-001", "PLN-001", model.RelationBelongsTo)
	addScopeRelation(t, engine, "TSK-001", "PHS-001", model.RelationBelongsTo)
	addScopeRelation(t, engine, "TSK-002", "PHS-001", model.RelationBelongsTo)
	addScopeRelation(t, engine, "TSK-001", "REQ-001", model.RelationCovers)
	addScopeRelation(t, engine, "TSK-002", "API-001", model.RelationCovers)

	planStatus := string(model.EntityStatusActive)
	if _, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "PLN-001", Status: &planStatus}); err != nil {
		t.Fatalf("activate plan: %v", err)
	}
	phaseNext, err := engine.PhaseNext(ctx, specgraph.PhaseNextRequest{})
	if err != nil {
		t.Fatalf("PhaseNext: %v", err)
	}
	queryScope, err := engine.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: "PHS-001"})
	if err != nil {
		t.Fatalf("QueryScope: %v", err)
	}
	queryIDs := make([]string, 0, len(queryScope.Entities))
	for _, entity := range queryScope.Entities {
		queryIDs = append(queryIDs, entity.ID)
	}
	sort.Strings(queryIDs)
	want := []string{"API-001", "REQ-001"}
	if phaseNext.Scope.Total != len(want) || !reflect.DeepEqual(queryIDs, want) {
		t.Fatalf("PhaseNext total=%d QueryScope=%v; want total=%d scope=%v", phaseNext.Scope.Total, queryIDs, len(want), want)
	}

	validation, err := engine.Validate(ctx, specgraph.ValidateRequest{Layer: "mapping", Checks: []string{"plan_coverage", "phase_satisfaction"}, Phase: "PHS-001"})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, issue := range validation.Issues {
		if issue.Check == "plan_coverage" {
			t.Fatalf("plan coverage disagrees with effective scope: %+v", issue)
		}
	}
	if len(validation.Satisfaction) != 1 {
		t.Fatalf("satisfaction reports=%d; want 1", len(validation.Satisfaction))
	}
	seedIDs := make([]string, 0, len(validation.Satisfaction[0].Items))
	for _, item := range validation.Satisfaction[0].Items {
		seedIDs = append(seedIDs, item.EntityID)
	}
	sort.Strings(seedIDs)
	if !reflect.DeepEqual(seedIDs, want) {
		t.Errorf("satisfaction seeds=%v; want %v", seedIDs, want)
	}
}

func TestRelationWriteRejectsMixedPhaseTaskMappings(t *testing.T) {
	t.Run("direct phase mapping after task parent", func(t *testing.T) {
		engine := openTestEngine(t)
		createScopeEntity(t, engine, "phase", "PHS-001")
		createScopeEntity(t, engine, "requirement", "REQ-001")
		createTask(t, engine, "TSK-001")
		addScopeRelation(t, engine, "TSK-001", "PHS-001", model.RelationBelongsTo)
		_, err := engine.AddRelation(context.Background(), specgraph.AddRelationRequest{From: "PHS-001", To: "REQ-001", Type: string(model.RelationCovers)})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("first task parent after direct phase mapping", func(t *testing.T) {
		engine := openTestEngine(t)
		createScopeEntity(t, engine, "phase", "PHS-001")
		createScopeEntity(t, engine, "requirement", "REQ-001")
		createTask(t, engine, "TSK-001")
		addScopeRelation(t, engine, "PHS-001", "REQ-001", model.RelationCovers)
		_, err := engine.AddRelation(context.Background(), specgraph.AddRelationRequest{From: "TSK-001", To: "PHS-001", Type: string(model.RelationBelongsTo)})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})
}

func createScopeEntity(t *testing.T, engine *specgraph.Engine, entityType, id string) {
	t.Helper()
	_, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{Type: entityType, ID: id, Title: id})
	if err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func addScopeRelation(t *testing.T, engine *specgraph.Engine, from, to string, relationType model.RelationType) {
	t.Helper()
	if _, err := engine.AddRelation(context.Background(), specgraph.AddRelationRequest{From: from, To: to, Type: string(relationType)}); err != nil {
		t.Fatalf("add %s %s->%s: %v", relationType, from, to, err)
	}
}

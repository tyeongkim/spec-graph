package specgraph_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestReviseCreatesChainAndDeprecatesPrior(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")

	title := "Query operations, split"
	result, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{
		ID:     "REQ-001",
		Title:  &title,
		Reason: "Split query ops from traversal",
	})
	if err != nil {
		t.Fatalf("ReviseEntity: %v", err)
	}

	if result.Revision.ID == "REQ-001" {
		t.Fatal("revision reused the superseded ID")
	}
	if result.Revision.Title != title {
		t.Errorf("revision title = %q; want %q", result.Revision.Title, title)
	}
	if result.Revision.Status != model.EntityStatusDraft {
		t.Errorf("revision status = %q; want %q", result.Revision.Status, model.EntityStatusDraft)
	}
	if result.Superseded.Status != model.EntityStatusDeprecated {
		t.Errorf("superseded status = %q; want %q", result.Superseded.Status, model.EntityStatusDeprecated)
	}

	supersedes := findRelation(t, eng, result.Revision.ID, "REQ-001", model.RelationSupersedes)
	var metadata map[string]any
	if err := json.Unmarshal(supersedes.Metadata, &metadata); err != nil {
		t.Fatalf("decode supersedes metadata: %v", err)
	}
	if metadata["reason"] != "Split query ops from traversal" {
		t.Errorf("supersedes reason = %v; want the request reason", metadata["reason"])
	}
}

func TestReviseCarriesOutboundAndInboundArchRelations(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")
	createEntity(t, eng, "requirement", "REQ-002", "Engine facade")
	createEntity(t, eng, "interface", "API-001", "Engine.QueryScope")
	addRelation(t, eng, "REQ-001", "REQ-002", "depends_on")
	addRelation(t, eng, "API-001", "REQ-001", "implements")

	result, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "Narrow the scope"})
	if err != nil {
		t.Fatalf("ReviseEntity: %v", err)
	}

	findRelation(t, eng, result.Revision.ID, "REQ-002", model.RelationDependsOn)
	findRelation(t, eng, "API-001", result.Revision.ID, model.RelationImplements)

	if relationExists(t, eng, "API-001", "REQ-001", model.RelationImplements) {
		t.Error("implements relation still points at the superseded entity")
	}
	if len(result.CarriedRelations) != 1 {
		t.Fatalf("carried %d relations; want 1", len(result.CarriedRelations))
	}
	if result.CarriedRelations[0].FromID != "API-001" {
		t.Errorf("carried relation source = %q; want API-001", result.CarriedRelations[0].FromID)
	}
}

func TestReviseRetainsResolvedPhaseMappings(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")
	resolvedPhase := setupResolvedPhaseDelivering(t, eng, "REQ-001")

	activePhase := "PHS-002"
	createEntity(t, eng, "phase", activePhase, "Phase 2")
	addRelation(t, eng, activePhase, "REQ-001", "covers")

	result, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "Requirement changed"})
	if err != nil {
		t.Fatalf("ReviseEntity: %v", err)
	}

	if !relationExists(t, eng, resolvedPhase, "REQ-001", model.RelationDelivers) {
		t.Error("resolved phase lost its delivers record of the superseded revision")
	}
	if relationExists(t, eng, resolvedPhase, result.Revision.ID, model.RelationDelivers) {
		t.Error("resolved phase was credited with delivering the new revision")
	}
	findRelation(t, eng, activePhase, result.Revision.ID, model.RelationCovers)

	if len(result.RetainedRelations) == 0 {
		t.Error("RetainedRelations is empty; want the resolved phase mappings")
	}
	for _, relation := range result.RetainedRelations {
		if relation.FromID != resolvedPhase {
			t.Errorf("retained relation source = %q; want %q", relation.FromID, resolvedPhase)
		}
	}
}

func TestReviseRetainedMappingsPassMappingConsistency(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")
	setupResolvedPhaseDelivering(t, eng, "REQ-001")

	if _, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "Requirement changed"}); err != nil {
		t.Fatalf("ReviseEntity: %v", err)
	}

	res, err := eng.Validate(ctx, specgraph.ValidateRequest{Checks: []string{"mapping_consistency"}})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, issue := range res.Issues {
		t.Errorf("unexpected mapping_consistency issue on %s: %s", issue.Entity, issue.Message)
	}
}

func TestReviseRejectsSecondRevisionOfSameEntity(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")

	if _, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "First revision"}); err != nil {
		t.Fatalf("first ReviseEntity: %v", err)
	}

	_, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "Forking revision"})
	assertErrorCode(t, err, specgraph.CodeConflict)
}

func TestReviseRejectsExecEntity(t *testing.T) {
	eng := openTestEngine(t)
	createEntity(t, eng, "phase", "PHS-001", "Phase 1")

	_, err := eng.ReviseEntity(context.Background(), specgraph.ReviseEntityRequest{ID: "PHS-001", Reason: "Rework"})
	assertErrorCode(t, err, specgraph.CodeInvalidInput)
}

func TestReviseRequiresReason(t *testing.T) {
	eng := openTestEngine(t)
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")

	_, err := eng.ReviseEntity(context.Background(), specgraph.ReviseEntityRequest{ID: "REQ-001"})
	assertErrorCode(t, err, specgraph.CodeInvalidInput)
}

func TestReviseMovesSymmetricRelationToRevision(t *testing.T) {
	eng := openTestEngine(t)
	ctx := context.Background()
	createEntity(t, eng, "requirement", "REQ-001", "Query operations")
	createEntity(t, eng, "requirement", "REQ-002", "Conflicting requirement")
	addRelation(t, eng, "REQ-001", "REQ-002", "conflicts_with")

	result, err := eng.ReviseEntity(ctx, specgraph.ReviseEntityRequest{ID: "REQ-001", Reason: "Resolve the conflict"})
	if err != nil {
		t.Fatalf("ReviseEntity: %v", err)
	}

	if !relationExists(t, eng, result.Revision.ID, "REQ-002", model.RelationConflictsWith) &&
		!relationExists(t, eng, "REQ-002", result.Revision.ID, model.RelationConflictsWith) {
		t.Error("conflicts_with relation did not move onto the revision")
	}
	if relationExists(t, eng, "REQ-001", "REQ-002", model.RelationConflictsWith) ||
		relationExists(t, eng, "REQ-002", "REQ-001", model.RelationConflictsWith) {
		t.Error("conflicts_with relation still references the superseded entity")
	}
}

func createEntity(t *testing.T, eng *specgraph.Engine, entityType, id, title string) {
	t.Helper()
	if _, err := eng.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
		Type: entityType, ID: id, Title: title,
	}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func addRelation(t *testing.T, eng *specgraph.Engine, from, to, relationType string) {
	t.Helper()
	if _, err := eng.AddRelation(context.Background(), specgraph.AddRelationRequest{
		From: from, To: to, Type: relationType,
	}); err != nil {
		t.Fatalf("add %s %s->%s: %v", relationType, from, to, err)
	}
}

// setupResolvedPhaseDelivering builds a resolved phase that delivers archID, the
// shape whose mappings must survive a revision of archID.
func setupResolvedPhaseDelivering(t *testing.T, eng *specgraph.Engine, archID string) string {
	t.Helper()
	ctx := context.Background()
	const phaseID = "PHS-001"

	createEntity(t, eng, "plan", "PLN-001", "Delivery plan")
	createEntity(t, eng, "phase", phaseID, "Phase 1")
	addRelation(t, eng, phaseID, "PLN-001", "belongs_to")
	addRelation(t, eng, phaseID, archID, "covers")
	addRelation(t, eng, phaseID, archID, "delivers")

	active := string(model.EntityStatusActive)
	if _, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "PLN-001", Status: &active}); err != nil {
		t.Fatalf("activate plan: %v", err)
	}
	if _, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: phaseID, Status: &active}); err != nil {
		t.Fatalf("activate phase: %v", err)
	}

	resolved := string(model.EntityStatusResolved)
	result, err := eng.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
		ID: phaseID, Status: &resolved, Force: true, Reason: "Accept for test setup",
	})
	if err != nil {
		t.Fatalf("resolve phase: %v", err)
	}
	if result.Entity.Status != model.EntityStatusResolved {
		t.Fatalf("phase status = %q; want resolved (outcome %q)", result.Entity.Status, result.Outcome)
	}
	return phaseID
}

func findRelation(t *testing.T, eng *specgraph.Engine, from, to string, relationType model.RelationType) model.Relation {
	t.Helper()
	relations, _, err := eng.ListRelations(context.Background(), specgraph.ListRelationsRequest{From: from})
	if err != nil {
		t.Fatalf("list relations from %s: %v", from, err)
	}
	for _, relation := range relations {
		if relation.ToID == to && relation.Type == relationType {
			return relation
		}
	}
	t.Fatalf("relation %s %s->%s not found", relationType, from, to)
	return model.Relation{}
}

func relationExists(t *testing.T, eng *specgraph.Engine, from, to string, relationType model.RelationType) bool {
	t.Helper()
	relations, _, err := eng.ListRelations(context.Background(), specgraph.ListRelationsRequest{From: from})
	if err != nil {
		t.Fatalf("list relations from %s: %v", from, err)
	}
	for _, relation := range relations {
		if relation.ToID == to && relation.Type == relationType {
			return true
		}
	}
	return false
}

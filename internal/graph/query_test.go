package graph

import (
	"errors"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestQueryScope_RejectsExistingNonPhaseID(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
	}}

	_, err := QueryScope(QueryScopeOptions{PhaseID: "REQ-001"}, rf, ef)
	if err == nil {
		t.Fatal("expected error for a non-phase entity")
	}

	var inputErr *model.ErrInvalidInput
	if !errors.As(err, &inputErr) {
		t.Errorf("error = %T (%v); want ErrInvalidInput", err, err)
	}
}

func TestQueryScope_DeduplicatesCoveredAndDeliveredEntity(t *testing.T) {
	covered := rel("PHS-001", "REQ-001", model.RelationCovers, 1.0)
	delivered := rel("PHS-001", "REQ-001", model.RelationDelivers, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"PHS-001": {covered, delivered},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"PHS-001": entity("PHS-001", model.EntityTypePhase),
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
	}}

	result, err := QueryScope(QueryScopeOptions{PhaseID: "PHS-001"}, rf, ef)
	if err != nil {
		t.Fatalf("QueryScope: %v", err)
	}

	if len(result.Relations) != 2 {
		t.Fatalf("len(relations) = %d; want 2", len(result.Relations))
	}
	if len(result.Entities) != 1 {
		t.Fatalf("len(entities) = %d; want 1", len(result.Entities))
	}
	if result.Entities[0].ID != "REQ-001" {
		t.Errorf("entities[0].id = %q; want REQ-001", result.Entities[0].ID)
	}
}

func TestQueryPath_PathSemantics(t *testing.T) {
	longStart := rel("API-001", "REQ-001", model.RelationImplements, 1.0)
	longMiddle := rel("TST-001", "API-001", model.RelationVerifies, 1.0)
	longEnd := rel("TST-001", "DEC-001", model.RelationVerifies, 1.0)
	shortStart := rel("REQ-001", "XCT-001", model.RelationConstrainedBy, 1.0)
	shortEnd := rel("XCT-001", "DEC-001", model.RelationReferences, 1.0)
	mappingOnly := rel("PHS-001", "REQ-001", model.RelationCovers, 1.0)

	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {longStart, shortStart, mappingOnly},
		"API-001": {longStart, longMiddle},
		"TST-001": {longMiddle, longEnd},
		"XCT-001": {shortStart, shortEnd},
		"DEC-001": {longEnd, shortEnd},
		"PHS-001": {mappingOnly},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
		"TST-001": entity("TST-001", model.EntityTypeTest),
		"XCT-001": entity("XCT-001", model.EntityTypeCrosscut),
		"DEC-001": entity("DEC-001", model.EntityTypeDecision),
		"PHS-001": entity("PHS-001", model.EntityTypePhase),
	}}

	result, err := QueryPath(QueryPathOptions{FromID: "REQ-001", ToID: "DEC-001"}, rf, ef)
	if err != nil {
		t.Fatalf("QueryPath: %v", err)
	}
	if !result.Found {
		t.Fatal("Found = false; want true")
	}

	wantPath := []PathNode{
		{EntityID: "REQ-001", Relation: ""},
		{EntityID: "XCT-001", Relation: model.RelationConstrainedBy},
		{EntityID: "DEC-001", Relation: model.RelationReferences},
	}
	if len(result.Path) != len(wantPath) {
		t.Fatalf("len(path) = %d; want %d", len(result.Path), len(wantPath))
	}
	for i, want := range wantPath {
		if result.Path[i].EntityID != want.EntityID {
			t.Errorf("path[%d].entity_id = %q; want %q", i, result.Path[i].EntityID, want.EntityID)
		}
		if result.Path[i].Relation != want.Relation {
			t.Errorf("path[%d].relation = %q; want %q", i, result.Path[i].Relation, want.Relation)
		}
	}

	mappingPath, err := QueryPath(QueryPathOptions{FromID: "PHS-001", ToID: "REQ-001"}, rf, ef)
	if err != nil {
		t.Fatalf("QueryPath mapping path: %v", err)
	}
	if !mappingPath.Found {
		t.Fatal("unrestricted mapping path not found")
	}

	arch := model.LayerArch
	filteredPath, err := QueryPath(QueryPathOptions{
		FromID: "PHS-001",
		ToID:   "REQ-001",
		Layer:  &arch,
	}, rf, ef)
	if err != nil {
		t.Fatalf("QueryPath arch layer: %v", err)
	}
	if filteredPath.Found {
		t.Error("arch-layer path found through a mapping relation")
	}
	if len(filteredPath.Path) != 0 {
		t.Errorf("len(filtered path) = %d; want 0", len(filteredPath.Path))
	}
}

func TestNeighbors_Depth0_CenterOnly(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {rel("REQ-001", "API-001", model.RelationImplements, 1.0)},
		"API-001": {rel("REQ-001", "API-001", model.RelationImplements, 1.0)},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
	}}

	result, err := Neighbors("REQ-001", 0, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Center != "REQ-001" {
		t.Errorf("center = %q; want REQ-001", result.Center)
	}
	if len(result.Entities) != 1 {
		t.Fatalf("len(entities) = %d; want 1", len(result.Entities))
	}
	if result.Entities[0].Entity.ID != "REQ-001" {
		t.Errorf("entities[0].id = %q; want REQ-001", result.Entities[0].Entity.ID)
	}
	if result.Entities[0].Depth != 0 {
		t.Errorf("entities[0].depth = %d; want 0", result.Entities[0].Depth)
	}
}

func TestNeighbors_Depth1_DirectNeighbors(t *testing.T) {
	r1 := rel("REQ-001", "API-001", model.RelationImplements, 1.0)
	r2 := rel("REQ-001", "DEC-001", model.RelationDependsOn, 1.0)
	r3 := rel("API-001", "TST-001", model.RelationVerifies, 1.0)

	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1, r2},
		"API-001": {r1, r3},
		"DEC-001": {r2},
		"TST-001": {r3},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
		"DEC-001": entity("DEC-001", model.EntityTypeDecision),
		"TST-001": entity("TST-001", model.EntityTypeTest),
	}}

	result, err := Neighbors("REQ-001", 1, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Center != "REQ-001" {
		t.Errorf("center = %q; want REQ-001", result.Center)
	}

	entityIDs := make(map[string]int)
	for _, ne := range result.Entities {
		entityIDs[ne.Entity.ID] = ne.Depth
	}

	if len(entityIDs) != 3 {
		t.Fatalf("len(entities) = %d; want 3 (REQ-001, API-001, DEC-001)", len(entityIDs))
	}
	if entityIDs["REQ-001"] != 0 {
		t.Errorf("REQ-001 depth = %d; want 0", entityIDs["REQ-001"])
	}
	if entityIDs["API-001"] != 1 {
		t.Errorf("API-001 depth = %d; want 1", entityIDs["API-001"])
	}
	if entityIDs["DEC-001"] != 1 {
		t.Errorf("DEC-001 depth = %d; want 1", entityIDs["DEC-001"])
	}
	if _, ok := entityIDs["TST-001"]; ok {
		t.Error("TST-001 should not be included at depth 1")
	}

	if len(result.Relations) != 2 {
		t.Errorf("len(relations) = %d; want 2", len(result.Relations))
	}
}

func TestNeighbors_Depth2_Transitive(t *testing.T) {
	r1 := rel("REQ-001", "API-001", model.RelationImplements, 1.0)
	r2 := rel("API-001", "TST-001", model.RelationVerifies, 1.0)

	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"API-001": {r1, r2},
		"TST-001": {r2},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
		"TST-001": entity("TST-001", model.EntityTypeTest),
	}}

	result, err := Neighbors("REQ-001", 2, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entityIDs := make(map[string]int)
	for _, ne := range result.Entities {
		entityIDs[ne.Entity.ID] = ne.Depth
	}

	if len(entityIDs) != 3 {
		t.Fatalf("len(entities) = %d; want 3", len(entityIDs))
	}
	if entityIDs["REQ-001"] != 0 {
		t.Errorf("REQ-001 depth = %d; want 0", entityIDs["REQ-001"])
	}
	if entityIDs["API-001"] != 1 {
		t.Errorf("API-001 depth = %d; want 1", entityIDs["API-001"])
	}
	if entityIDs["TST-001"] != 2 {
		t.Errorf("TST-001 depth = %d; want 2", entityIDs["TST-001"])
	}

	if len(result.Relations) != 2 {
		t.Errorf("len(relations) = %d; want 2", len(result.Relations))
	}
}

func TestNeighbors_BidirectionalTraversal(t *testing.T) {
	r1 := rel("API-001", "REQ-001", model.RelationImplements, 1.0)

	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"API-001": {r1},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
	}}

	result, err := Neighbors("REQ-001", 1, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entityIDs := make(map[string]int)
	for _, ne := range result.Entities {
		entityIDs[ne.Entity.ID] = ne.Depth
	}

	if _, ok := entityIDs["API-001"]; !ok {
		t.Error("API-001 should be reachable via reverse edge at depth 1")
	}
}

func TestNeighbors_NonexistentEntity(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{}}
	ef := &mockEF{entities: map[string]model.Entity{}}

	_, err := Neighbors("NONEXIST-001", 1, rf, ef)
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
}

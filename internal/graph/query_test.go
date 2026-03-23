package graph

import (
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

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

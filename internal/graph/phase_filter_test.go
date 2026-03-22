package graph

import (
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestPhaseFilteredEntityFetcher_FiltersToPhaseScope(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
			"API-001": {{ID: 2, FromID: "API-001", ToID: "PHS-001", Type: model.RelationDeliveredIn}},
			"PHS-001": {
				{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
				{ID: 2, FromID: "API-001", ToID: "PHS-001", Type: model.RelationDeliveredIn},
			},
		},
	}

	filtered, err := newPhaseFilteredEntityFetcher(ef, rf, "PHS-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entities, err := filtered.List(EntityListFilters{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	ids := make(map[string]bool)
	for _, e := range entities {
		ids[e.ID] = true
	}

	if !ids["PHS-001"] {
		t.Error("expected PHS-001 (phase itself) in result")
	}
	if !ids["REQ-001"] {
		t.Error("expected REQ-001 (planned_in) in result")
	}
	if !ids["API-001"] {
		t.Error("expected API-001 (delivered_in) in result")
	}
	if ids["REQ-002"] {
		t.Error("expected REQ-002 (not linked to phase) to be excluded")
	}
	if len(entities) != 3 {
		t.Errorf("got %d entities; want 3", len(entities))
	}
}

func TestPhaseFilteredEntityFetcher_Get_AllowsAnyID(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
			"PHS-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
		},
	}

	filtered, err := newPhaseFilteredEntityFetcher(ef, rf, "PHS-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e, err := filtered.Get("REQ-001")
	if err != nil {
		t.Errorf("Get(REQ-001) unexpected error: %v", err)
	}
	if e.ID != "REQ-001" {
		t.Errorf("got ID %q; want REQ-001", e.ID)
	}

	e, err = filtered.Get("REQ-002")
	if err != nil {
		t.Errorf("Get(REQ-002) should succeed (Get is permissive): %v", err)
	}
	if e.ID != "REQ-002" {
		t.Errorf("got ID %q; want REQ-002", e.ID)
	}
}

func TestPhaseFilteredEntityFetcher_EmptyPhase(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{},
	}

	filtered, err := newPhaseFilteredEntityFetcher(ef, rf, "PHS-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entities, err := filtered.List(EntityListFilters{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("got %d entities; want 1 (phase itself only)", len(entities))
	}
	if entities[0].ID != "PHS-001" {
		t.Errorf("got entity %q; want PHS-001", entities[0].ID)
	}
}

func TestPhaseFilteredRelationFetcher_FiltersToPhaseScope(t *testing.T) {
	allowed := map[string]bool{
		"PHS-001": true,
		"REQ-001": true,
		"API-001": true,
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
				{ID: 2, FromID: "REQ-001", ToID: "REQ-002", Type: model.RelationDependsOn},
			},
		},
	}

	filtered := newPhaseFilteredRelationFetcher(rf, allowed)

	rels, err := filtered.GetByEntity("REQ-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rels) != 1 {
		t.Fatalf("got %d relations; want 1 (only in-scope)", len(rels))
	}
	if rels[0].ToID != "PHS-001" {
		t.Errorf("got ToID %q; want PHS-001", rels[0].ToID)
	}
}

func TestPhaseFilteredRelationFetcher_EmptyPhase(t *testing.T) {
	allowed := map[string]bool{
		"PHS-001": true,
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"PHS-001": {},
		},
	}

	filtered := newPhaseFilteredRelationFetcher(rf, allowed)

	rels, err := filtered.GetByEntity("PHS-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rels) != 0 {
		t.Errorf("got %d relations; want 0", len(rels))
	}
}

func TestPhaseFilteredRelationFetcher_OutOfScopeEntityReturnsEmpty(t *testing.T) {
	allowed := map[string]bool{
		"PHS-001": true,
		"REQ-001": true,
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-002": {{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationDependsOn}},
		},
	}

	filtered := newPhaseFilteredRelationFetcher(rf, allowed)

	rels, err := filtered.GetByEntity("REQ-002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rels) != 0 {
		t.Errorf("got %d relations; want 0 (out-of-scope entity)", len(rels))
	}
}

func TestPhaseFilteredRelationFetcher_PreservesAllRelationTypes(t *testing.T) {
	allowed := map[string]bool{
		"PHS-001": true,
		"REQ-001": true,
		"DEC-001": true,
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
				{ID: 2, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn},
			},
		},
	}

	filtered := newPhaseFilteredRelationFetcher(rf, allowed)

	rels, err := filtered.GetByEntity("REQ-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rels) != 2 {
		t.Errorf("got %d relations; want 2 (all in-scope relation types preserved)", len(rels))
	}
}

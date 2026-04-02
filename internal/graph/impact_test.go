package graph

import (
	"math"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

type mockRF struct {
	relations map[string][]model.Relation
}

func (m *mockRF) GetByEntity(entityID string) ([]model.Relation, error) {
	return m.relations[entityID], nil
}

type mockEF struct {
	entities map[string]model.Entity
}

func (m *mockEF) Get(id string) (model.Entity, error) {
	e, ok := m.entities[id]
	if !ok {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return e, nil
}

func (m *mockEF) List(_ EntityListFilters) ([]model.Entity, error) {
	out := make([]model.Entity, 0, len(m.entities))
	for _, e := range m.entities {
		out = append(out, e)
	}
	return out, nil
}

func rel(from, to string, rt model.RelationType, weight float64) model.Relation {
	return model.Relation{FromID: from, ToID: to, Type: rt, Weight: weight}
}

func entity(id string, t model.EntityType) model.Entity {
	return model.Entity{ID: id, Type: t, Status: model.EntityStatusActive}
}

func findAffected(result *ImpactResult, id string) *AffectedEntity {
	for i := range result.Affected {
		if result.Affected[i].ID == id {
			return &result.Affected[i]
		}
	}
	return nil
}

func approx(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func strPtr(s string) *string { return &s }

func sevPtr(s Severity) *Severity { return &s }

func TestImpact_SingleSource_DirectNeighbors(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Affected) != 1 {
		t.Fatalf("expected 1 affected, got %d", len(result.Affected))
	}

	a := result.Affected[0]
	if a.ID != "API-005" {
		t.Errorf("expected API-005, got %s", a.ID)
	}
	if a.Depth != 1 {
		t.Errorf("expected depth 1, got %d", a.Depth)
	}
	if !approx(a.Impact.Structural, 0.9) {
		t.Errorf("structural: got %f, want 0.9", a.Impact.Structural)
	}
	if !approx(a.Impact.Behavioral, 0.8) {
		t.Errorf("behavioral: got %f, want 0.8", a.Impact.Behavioral)
	}
	if !approx(a.Impact.Planning, 0.4) {
		t.Errorf("planning: got %f, want 0.4", a.Impact.Planning)
	}
}

func TestImpact_SingleSource_MultiHop(t *testing.T) {
	r1 := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	r2 := rel("TST-012", "API-005", model.RelationVerifies, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"API-005": {r1, r2},
		"TST-012": {r2},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"TST-012": entity("TST-012", model.EntityTypeTest),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	api := findAffected(result, "API-005")
	tst := findAffected(result, "TST-012")
	if api == nil || tst == nil {
		t.Fatalf("expected both API-005 and TST-012 affected, got %d affected", len(result.Affected))
	}

	if tst.Depth != 2 {
		t.Errorf("TST-012 depth: got %d, want 2", tst.Depth)
	}

	// TST-012 reached via API-005 → verifies is ForwardReverseWeak, reverse direction
	// structural: 0.9 * 0.4 * 0.5 = 0.18
	wantS := 0.9 * 0.4 * ReverseWeakFactor
	if !approx(tst.Impact.Structural, wantS) {
		t.Errorf("TST-012 structural: got %f, want %f", tst.Impact.Structural, wantS)
	}
}

func TestImpact_MultipleSources(t *testing.T) {
	r1 := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	r2 := rel("DEC-003", "API-005", model.RelationDependsOn, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"DEC-003": {r2},
		"API-005": {r1, r2},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"DEC-003": entity("DEC-003", model.EntityTypeDecision),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"REQ-001", "DEC-003"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	api := findAffected(result, "API-005")
	if api == nil {
		t.Fatal("expected API-005 in affected")
	}

	// implements structural=0.9, depends_on structural=0.8 → best is 0.9
	if api.Impact.Structural < 0.8 {
		t.Errorf("expected structural >= 0.8, got %f", api.Impact.Structural)
	}
}

func TestImpact_BidirectionalPropagation(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"API-005"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := findAffected(result, "REQ-001")
	if req == nil {
		t.Fatal("expected REQ-001 affected via bidirectional implements")
	}
	if !approx(req.Impact.Structural, 0.9) {
		t.Errorf("structural: got %f, want 0.9", req.Impact.Structural)
	}
}

func TestImpact_ReverseWeakDampening(t *testing.T) {
	// verifies: TST-012 → REQ-001 (forward from TST-012)
	ver := rel("TST-012", "REQ-001", model.RelationVerifies, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"TST-012": {ver},
		"REQ-001": {ver},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"TST-012": entity("TST-012", model.EntityTypeTest),
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
	}}

	// Forward: source=TST-012 → REQ-001 gets full weight
	fwd, err := Impact([]string{"TST-012"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	fwdReq := findAffected(fwd, "REQ-001")
	if fwdReq == nil {
		t.Fatal("expected REQ-001 affected in forward")
	}

	// Reverse: source=REQ-001 → TST-012 gets ×0.5
	rev, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	revTst := findAffected(rev, "TST-012")
	if revTst == nil {
		t.Fatal("expected TST-012 affected in reverse")
	}

	// verifies behavioral: forward=0.8, reverse=0.8*0.5=0.4
	if !approx(fwdReq.Impact.Behavioral, 0.8) {
		t.Errorf("forward behavioral: got %f, want 0.8", fwdReq.Impact.Behavioral)
	}
	if !approx(revTst.Impact.Behavioral, 0.8*ReverseWeakFactor) {
		t.Errorf("reverse behavioral: got %f, want %f", revTst.Impact.Behavioral, 0.8*ReverseWeakFactor)
	}
}

func TestImpact_CycleProtection(t *testing.T) {
	rAB := rel("A", "B", model.RelationImplements, 1.0)
	rBC := rel("B", "C", model.RelationImplements, 1.0)
	rCA := rel("C", "A", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"A": {rAB, rCA},
		"B": {rAB, rBC},
		"C": {rBC, rCA},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"A": entity("A", model.EntityTypeRequirement),
		"B": entity("B", model.EntityTypeInterface),
		"C": entity("C", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"A"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Affected) != 2 {
		t.Errorf("expected 2 affected (B, C), got %d", len(result.Affected))
	}
}

func TestImpact_NoRelations(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Affected) != 0 {
		t.Errorf("expected 0 affected, got %d", len(result.Affected))
	}
}

func TestImpact_FollowFilter(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	dep := rel("REQ-001", "DEC-003", model.RelationDependsOn, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl, dep},
		"API-005": {impl},
		"DEC-003": {dep},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"DEC-003": entity("DEC-003", model.EntityTypeDecision),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{
		Follow: []model.RelationType{model.RelationImplements},
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Affected) != 1 {
		t.Fatalf("expected 1 affected, got %d", len(result.Affected))
	}
	if result.Affected[0].ID != "API-005" {
		t.Errorf("expected API-005, got %s", result.Affected[0].ID)
	}
}

func TestImpact_DimensionFilter(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{
		Dimension: strPtr("structural"),
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := findAffected(result, "API-005")
	if a == nil {
		t.Fatal("expected API-005 affected")
	}
	if !approx(a.Impact.Structural, 0.9) {
		t.Errorf("structural: got %f, want 0.9", a.Impact.Structural)
	}
	if a.Impact.Behavioral != 0 {
		t.Errorf("behavioral should be 0, got %f", a.Impact.Behavioral)
	}
	if a.Impact.Planning != 0 {
		t.Errorf("planning should be 0, got %f", a.Impact.Planning)
	}
}

func TestImpact_MinSeverityFilter(t *testing.T) {
	// implements: structural=0.9 (high), references: structural=0.1 (low)
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	refs := rel("REQ-001", "DEC-003", model.RelationReferences, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl, refs},
		"API-005": {impl},
		"DEC-003": {refs},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"DEC-003": entity("DEC-003", model.EntityTypeDecision),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{
		MinSeverity: sevPtr(SeverityHigh),
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// API-005 has max score 0.9 (high), DEC-003 has max score 0.1 (low) → filtered
	if len(result.Affected) != 1 {
		t.Fatalf("expected 1 affected (high only), got %d", len(result.Affected))
	}
	if result.Affected[0].ID != "API-005" {
		t.Errorf("expected API-005, got %s", result.Affected[0].ID)
	}
}

func TestImpact_RelationWeight(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 2.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := findAffected(result, "API-005")
	if a == nil {
		t.Fatal("expected API-005 affected")
	}
	// structural: 1.0 * 0.9 * 2.0 = 1.8
	if !approx(a.Impact.Structural, 1.8) {
		t.Errorf("structural: got %f, want 1.8", a.Impact.Structural)
	}
}

func TestImpact_PathTracking(t *testing.T) {
	r1 := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	r2 := rel("TST-012", "API-005", model.RelationVerifies, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"API-005": {r1, r2},
		"TST-012": {r2},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"TST-012": entity("TST-012", model.EntityTypeTest),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	api := findAffected(result, "API-005")
	if api == nil {
		t.Fatal("expected API-005")
	}
	if len(api.Path) != 2 || api.Path[0] != "REQ-001" || api.Path[1] != "API-005" {
		t.Errorf("API-005 path: got %v, want [REQ-001 API-005]", api.Path)
	}

	tst := findAffected(result, "TST-012")
	if tst == nil {
		t.Fatal("expected TST-012")
	}
	if len(tst.Path) != 3 || tst.Path[0] != "REQ-001" || tst.Path[1] != "API-005" || tst.Path[2] != "TST-012" {
		t.Errorf("TST-012 path: got %v, want [REQ-001 API-005 TST-012]", tst.Path)
	}
}

func TestImpact_RelationChain(t *testing.T) {
	r1 := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	r2 := rel("TST-012", "API-005", model.RelationVerifies, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {r1},
		"API-005": {r1, r2},
		"TST-012": {r2},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"TST-012": entity("TST-012", model.EntityTypeTest),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tst := findAffected(result, "TST-012")
	if tst == nil {
		t.Fatal("expected TST-012")
	}
	if len(tst.RelationChain) != 2 {
		t.Fatalf("chain length: got %d, want 2", len(tst.RelationChain))
	}
	if tst.RelationChain[0] != model.RelationImplements {
		t.Errorf("chain[0]: got %s, want implements", tst.RelationChain[0])
	}
	if tst.RelationChain[1] != model.RelationVerifies {
		t.Errorf("chain[1]: got %s, want verifies", tst.RelationChain[1])
	}
}

func TestImpact_OverallSeverity(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := findAffected(result, "API-005")
	if a == nil {
		t.Fatal("expected API-005")
	}
	// max(0.9, 0.8, 0.4) = 0.9 → high
	if a.Overall != SeverityHigh {
		t.Errorf("overall: got %s, want high", a.Overall)
	}
}

func TestImpact_SummaryAggregation(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	refs := rel("REQ-001", "DEC-003", model.RelationReferences, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl, refs},
		"API-005": {impl},
		"DEC-003": {refs},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-005": entity("API-005", model.EntityTypeInterface),
		"DEC-003": entity("DEC-003", model.EntityTypeDecision),
	}}

	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.Total != 2 {
		t.Errorf("total: got %d, want 2", result.Summary.Total)
	}
	if result.Summary.ByType[model.EntityTypeInterface] != 1 {
		t.Errorf("by_type interface: got %d, want 1", result.Summary.ByType[model.EntityTypeInterface])
	}
	if result.Summary.ByType[model.EntityTypeDecision] != 1 {
		t.Errorf("by_type decision: got %d, want 1", result.Summary.ByType[model.EntityTypeDecision])
	}

	totalBySeverity := 0
	for _, c := range result.Summary.ByImpact {
		totalBySeverity += c
	}
	if totalBySeverity != 2 {
		t.Errorf("by_impact total: got %d, want 2", totalBySeverity)
	}
}

func TestImpact_EntityNotFound(t *testing.T) {
	impl := rel("REQ-001", "API-005", model.RelationImplements, 1.0)
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {impl},
		"API-005": {impl},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		// API-005 intentionally missing
	}}

	_, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err == nil {
		t.Fatal("expected error for missing entity, got nil")
	}
}

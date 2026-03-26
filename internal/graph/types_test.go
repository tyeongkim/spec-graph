package graph

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestImpactResultJSON(t *testing.T) {
	result := ImpactResult{
		Sources: []string{"REQ-001"},
		Affected: []AffectedEntity{
			{
				ID:            "API-005",
				Type:          model.EntityTypeInterface,
				Depth:         1,
				Path:          []string{"REQ-001", "API-005"},
				RelationChain: []model.RelationType{model.RelationImplements},
				Impact:        DimensionScores{Structural: 0.9, Behavioral: 0.5, Planning: 0.2},
				Overall:       SeverityHigh,
				Reason:        "direct implementor",
			},
		},
		Summary: ImpactSummary{
			Total:    1,
			ByType:   map[model.EntityType]int{model.EntityTypeInterface: 1},
			ByImpact: map[Severity]int{SeverityHigh: 1},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := m["sources"]; !ok {
		t.Error("missing field: sources")
	}

	affected, ok := m["affected"].([]interface{})
	if !ok || len(affected) == 0 {
		t.Fatal("missing or empty field: affected")
	}

	entity, ok := affected[0].(map[string]interface{})
	if !ok {
		t.Fatal("affected[0] is not an object")
	}

	for _, field := range []string{"id", "type", "depth", "path", "relation_chain", "impact", "overall", "reason"} {
		if _, ok := entity[field]; !ok {
			t.Errorf("affected entity missing field: %s", field)
		}
	}

	summary, ok := m["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("missing field: summary")
	}

	for _, field := range []string{"total", "by_type", "by_impact"} {
		if _, ok := summary[field]; !ok {
			t.Errorf("summary missing field: %s", field)
		}
	}
}

func TestInterfaceSatisfaction(t *testing.T) {
	var _ RelationFetcher = (*mockRelationFetcher)(nil)
	var _ EntityFetcher = (*mockEntityFetcher)(nil)
}

type mockRelationFetcher struct{}

func (m *mockRelationFetcher) GetByEntity(_ string) ([]model.Relation, error) {
	return nil, nil
}

type mockEntityFetcher struct{}

func (m *mockEntityFetcher) Get(_ string) (model.Entity, error) {
	return model.Entity{}, nil
}

func (m *mockEntityFetcher) List(_ EntityListFilters) ([]model.Entity, error) {
	return nil, nil
}

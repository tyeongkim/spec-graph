package graph

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestImpact_PreservesStrongestDimensionPaths(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {
			rel("REQ-001", "API-001", model.RelationImplements, 1),
			rel("REQ-001", "API-001", model.RelationTriggers, .95),
			rel("REQ-001", "API-001", model.RelationDelivers, .95),
		},
		"API-001": {
			rel("API-001", "STT-001", model.RelationTriggers, 1),
			rel("API-001", "PHS-001", model.RelationBlocks, 1),
		},
	}}
	ef := &mockEF{entities: map[string]model.Entity{
		"API-001": entity("API-001", model.EntityTypeInterface),
		"STT-001": entity("STT-001", model.EntityTypeState),
		"PHS-001": entity("PHS-001", model.EntityTypePhase),
	}}
	result, err := Impact([]string{"REQ-001"}, ImpactOptions{}, rf, ef)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     string
		scores DimensionScores
		chain  []model.RelationType
	}{
		{"API-001", DimensionScores{.9, .855, .855}, []model.RelationType{model.RelationImplements}},
		{"STT-001", DimensionScores{.54, .7695, .171}, []model.RelationType{model.RelationTriggers, model.RelationTriggers}},
		{"PHS-001", DimensionScores{.18, .171, .684}, []model.RelationType{model.RelationDelivers, model.RelationBlocks}},
	} {
		a := findAffected(result, tc.id)
		if a == nil {
			t.Fatalf("missing affected entity %s", tc.id)
		}
		if !approx(a.Impact.Structural, tc.scores.Structural) || !approx(a.Impact.Behavioral, tc.scores.Behavioral) || !approx(a.Impact.Planning, tc.scores.Planning) {
			t.Errorf("%s scores = %+v; want %+v", tc.id, a.Impact, tc.scores)
		}
		if !slices.Equal(a.RelationChain, tc.chain) || a.Depth != len(tc.chain) || len(a.Path) != a.Depth+1 || a.Path[0] != "REQ-001" || a.Path[len(a.Path)-1] != tc.id || a.Reason != ReasonTemplates[tc.chain[len(tc.chain)-1]] {
			t.Errorf("%s inconsistent path evidence: %+v; want chain %v", tc.id, a, tc.chain)
		}
	}
}

type failingEntityFetcher struct {
	EntityFetcher
	id  string
	err error
}

func (f failingEntityFetcher) Get(id string) (model.Entity, error) {
	if id == f.id {
		return model.Entity{}, f.err
	}
	return f.EntityFetcher.Get(id)
}

type failingRelationFetcher struct {
	RelationFetcher
	id  string
	err error
}

func (f failingRelationFetcher) GetByEntity(id string) ([]model.Relation, error) {
	if id == f.id {
		return nil, f.err
	}
	return f.RelationFetcher.GetByEntity(id)
}

func TestTraversal_EntityFetchFailures(t *testing.T) {
	rf := &mockRF{relations: map[string][]model.Relation{
		"REQ-001": {rel("REQ-001", "API-001", model.RelationImplements, 1)},
	}}
	base := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"DEC-001": entity("DEC-001", model.EntityTypeDecision),
	}}
	for _, traversal := range []string{"path", "neighbors"} {
		for _, missing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing=%t", traversal, missing), func(t *testing.T) {
				failure := errors.New("storage unavailable")
				if missing {
					failure = fmt.Errorf("lookup: %w", &model.ErrEntityNotFound{ID: "API-001"})
				}
				ef := failingEntityFetcher{EntityFetcher: base, id: "API-001", err: failure}
				var err error
				if traversal == "path" {
					_, err = QueryPath(QueryPathOptions{FromID: "REQ-001", ToID: "DEC-001"}, rf, ef)
				} else {
					_, err = Neighbors("REQ-001", 1, rf, ef)
				}
				if missing {
					if err != nil {
						t.Fatalf("dangling entity: %v", err)
					}
				} else if !errors.Is(err, failure) {
					t.Fatalf("error = %v; want wrapped storage failure", err)
				}
			})
		}
	}
}

func TestNeighbors_LastLayerRelationFetchFailure(t *testing.T) {
	failure := errors.New("storage unavailable")
	rf := failingRelationFetcher{
		RelationFetcher: &mockRF{relations: map[string][]model.Relation{
			"REQ-001": {rel("REQ-001", "API-001", model.RelationImplements, 1)},
		}},
		id: "API-001", err: failure,
	}
	ef := &mockEF{entities: map[string]model.Entity{
		"REQ-001": entity("REQ-001", model.EntityTypeRequirement),
		"API-001": entity("API-001", model.EntityTypeInterface),
	}}
	_, err := Neighbors("REQ-001", 1, rf, ef)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v; want wrapped storage failure", err)
	}
}

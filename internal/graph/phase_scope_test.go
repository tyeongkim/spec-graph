package graph

import (
	"reflect"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestEffectivePhaseScopeTaskManaged(t *testing.T) {
	relations := []model.Relation{
		rel("TSK-001", "PHS-001", model.RelationBelongsTo, 1),
		rel("TSK-002", "PHS-001", model.RelationBelongsTo, 1),
		rel("TSK-001", "REQ-001", model.RelationCovers, 1),
		rel("TSK-002", "API-001", model.RelationCovers, 1),
		rel("TSK-002", "REQ-001", model.RelationCovers, 1),
		rel("TSK-001", "REQ-001", model.RelationDelivers, 1),
		rel("PHS-001", "DEC-001", model.RelationCovers, 1),
	}
	rf := relationsByEntity(relationSliceFetcher{}, relations)

	scope, err := EffectivePhaseScope("PHS-001", rf)
	if err != nil {
		t.Fatalf("EffectivePhaseScope: %v", err)
	}
	if !scope.TaskManaged {
		t.Fatal("TaskManaged = false; want true")
	}
	if want := []string{"API-001", "REQ-001"}; !reflect.DeepEqual(scope.Covered, want) {
		t.Errorf("Covered = %v; want %v", scope.Covered, want)
	}
	if want := []string{"REQ-001"}; !reflect.DeepEqual(scope.Delivered, want) {
		t.Errorf("Delivered = %v; want %v", scope.Delivered, want)
	}
}

func TestEffectivePhaseScopeLegacyTaskless(t *testing.T) {
	relations := []model.Relation{
		rel("PHS-001", "REQ-001", model.RelationCovers, 1),
		rel("PHS-001", "API-001", model.RelationCovers, 1),
		rel("PHS-001", "API-001", model.RelationDelivers, 1),
	}
	rf := relationsByEntity(relationSliceFetcher{}, relations)

	scope, err := EffectivePhaseScope("PHS-001", rf)
	if err != nil {
		t.Fatalf("EffectivePhaseScope: %v", err)
	}
	if scope.TaskManaged {
		t.Fatal("TaskManaged = true; want false")
	}
	if want := []string{"REQ-001", "API-001"}; !reflect.DeepEqual(scope.Covered, want) {
		t.Errorf("Covered = %v; want %v", scope.Covered, want)
	}
	if want := []string{"API-001"}; !reflect.DeepEqual(scope.Delivered, want) {
		t.Errorf("Delivered = %v; want %v", scope.Delivered, want)
	}
}

type relationSliceFetcher map[string][]model.Relation

func relationsByEntity(fetcher relationSliceFetcher, relations []model.Relation) relationSliceFetcher {
	for _, relation := range relations {
		fetcher[relation.FromID] = append(fetcher[relation.FromID], relation)
		fetcher[relation.ToID] = append(fetcher[relation.ToID], relation)
	}
	return fetcher
}

func (f relationSliceFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	return f[entityID], nil
}

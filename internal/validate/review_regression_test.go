package validate

import (
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestExecCyclesCombinePhaseOrderingRelations(t *testing.T) {
	for _, tc := range []struct {
		name       string
		relations  []model.Relation
		wantIssues int
	}{
		{"precedes cycle", []model.Relation{rel(1, "PHS-1", "PHS-2", model.RelationPrecedes), rel(2, "PHS-2", "PHS-1", model.RelationPrecedes)}, 2},
		{"mixed cycle", []model.Relation{rel(1, "PHS-1", "PHS-2", model.RelationPrecedes), rel(2, "PHS-2", "PHS-1", model.RelationBlocks)}, 2},
		{"parallel ordering constraints", []model.Relation{rel(1, "PHS-1", "PHS-2", model.RelationPrecedes), rel(2, "PHS-1", "PHS-2", model.RelationBlocks)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ef := newMockEntityFetcher(execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil), execEntity("PHS-2", model.EntityTypePhase, model.EntityStatusActive, nil))
			issues := checkExecCycles(newMockRelationFetcher(tc.relations...), ef)
			if len(issues) != tc.wantIssues {
				t.Fatalf("issues=%+v; want %d", issues, tc.wantIssues)
			}
		})
	}
}

func TestTaskManagedPhaseRejectsDirectMappingsBeforeTasksHaveScope(t *testing.T) {
	for _, relationType := range []model.RelationType{model.RelationCovers, model.RelationDelivers} {
		t.Run(string(relationType), func(t *testing.T) {
			ef := newMockEntityFetcher(execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil), execEntity("TSK-1", model.EntityTypeTask, model.EntityStatusActive, nil), archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive))
			rf := newMockRelationFetcher(rel(1, "TSK-1", "PHS-1", model.RelationBelongsTo), rel(2, "PHS-1", "REQ-1", relationType))
			issues := checkTaskScope(rf, ef)
			for _, issue := range issues {
				if issue.Entity == "PHS-1" && issue.Check == "task_scope" && issue.Severity == SeverityHigh {
					return
				}
			}
			t.Fatalf("issues=%+v; want a high task_scope issue for phase", issues)
		})
	}
}

func TestScopedValidationPreservesStorageFailures(t *testing.T) {
	phaseID := "PHS-1"
	for _, tc := range []struct {
		name string
		opts ValidateOptions
		rf   RelationFetcher
		ef   EntityFetcher
	}{
		{"entity list", ValidateOptions{EntityID: "REQ-1", Checks: []string{"orphans"}}, newMockRelationFetcher(), listFailingEntityFetcher{}},
		{"phase list", ValidateOptions{Phase: &phaseID, Checks: []string{"orphans"}}, newMockRelationFetcher(), selectiveListFailureFetcher{newMockEntityFetcher(execEntity(phaseID, model.EntityTypePhase, model.EntityStatusActive, nil))}},
		{"entity relation outside scope", ValidateOptions{EntityID: "REQ-1", Checks: []string{"orphans"}}, failingRelationFetcher{}, newMockEntityFetcher(archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Validate(tc.opts, tc.rf, tc.ef)
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid || !mentionsStorageError(result.Issues) {
				t.Fatalf("storage failure lost: %+v", result)
			}
		})
	}
}

type selectiveListFailureFetcher struct{ EntityFetcher }

func (f selectiveListFailureFetcher) List(filters EntityListFilters) ([]model.Entity, error) {
	if filters.Layer != nil && *filters.Layer == model.LayerArch {
		return nil, errStorage
	}
	return f.EntityFetcher.List(filters)
}

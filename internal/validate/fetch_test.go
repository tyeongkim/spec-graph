package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

var errStorage = errors.New("index is corrupt")

// failingRelationFetcher fails every relation read, standing in for a corrupt or
// unreadable index.
type failingRelationFetcher struct{}

func (failingRelationFetcher) GetByEntity(string) ([]model.Relation, error) {
	return nil, errStorage
}

// failingEntityFetcher serves List from a working set but fails every Get, so a
// check gets its subjects and then fails to read their neighbours.
type failingEntityFetcher struct {
	entities []model.Entity
}

func (f failingEntityFetcher) Get(string) (model.Entity, error) {
	return model.Entity{}, errStorage
}

func (f failingEntityFetcher) List(filters EntityListFilters) ([]model.Entity, error) {
	var result []model.Entity
	for _, e := range f.entities {
		if filters.Type != nil && e.Type != *filters.Type {
			continue
		}
		if filters.Status != nil && e.Status != *filters.Status {
			continue
		}
		if filters.Layer != nil && e.Layer != *filters.Layer {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}

// listFailingEntityFetcher fails every List, so a check cannot obtain subjects.
type listFailingEntityFetcher struct{}

func (listFailingEntityFetcher) Get(id string) (model.Entity, error) {
	return model.Entity{}, &model.ErrEntityNotFound{ID: id}
}

func (listFailingEntityFetcher) List(EntityListFilters) ([]model.Entity, error) {
	return nil, errStorage
}

// A storage failure must never be reported as a clean graph. Each check below
// previously swallowed the error and returned zero issues.
func TestChecksReportStorageFailureInsteadOfSuccess(t *testing.T) {
	activeReq := model.Entity{
		ID: "REQ-001", Type: model.EntityTypeRequirement,
		Layer: model.LayerArch, Status: model.EntityStatusActive,
		Metadata: []byte(`{}`),
	}
	activePhase := model.Entity{
		ID: "PHS-001", Type: model.EntityTypePhase,
		Layer: model.LayerExec, Status: model.EntityStatusActive,
		Metadata: []byte(`{}`),
	}
	activeTask := model.Entity{
		ID: "TSK-001", Type: model.EntityTypeTask,
		Layer: model.LayerExec, Status: model.EntityStatusActive,
		Metadata: []byte(`{}`),
	}
	activeChange := model.Entity{
		ID: "CHG-001", Type: model.EntityTypeChange,
		Layer: model.LayerExec, Status: model.EntityStatusActive,
		Metadata: []byte(`{}`),
	}

	tests := []struct {
		name  string
		check string
		run   func(rf RelationFetcher, ef EntityFetcher) []ValidationIssue
		ef    EntityFetcher
	}{
		{"orphans", "orphans", checkOrphans, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"coverage", "coverage", checkCoverage, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"cycles", "cycles", checkCycles, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"conflicts", "conflicts", checkConflicts, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"invalid_edges", "invalid_edges", checkInvalidEdges, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"superseded_refs", "superseded_refs", checkSupersededRefs, failingEntityFetcher{entities: []model.Entity{activeReq}}},
		{"unresolved", "unresolved", checkUnresolved, failingEntityFetcher{entities: []model.Entity{
			{ID: "QST-001", Type: model.EntityTypeQuestion, Layer: model.LayerArch, Status: model.EntityStatusActive, Metadata: []byte(`{}`)},
		}}},
		{"phase_order", "phase_order", checkPhaseOrder, failingEntityFetcher{entities: []model.Entity{activePhase}}},
		{"orphan_phases", "orphan_phases", checkOrphanPhases, failingEntityFetcher{entities: []model.Entity{activePhase}}},
		{"exec_cycles", "exec_cycles", checkExecCycles, failingEntityFetcher{entities: []model.Entity{activePhase}}},
		{"orphan_changes", "orphan_changes", checkOrphanChanges, failingEntityFetcher{entities: []model.Entity{activeChange}}},
		{"task_graph", "task_graph", checkTaskGraph, failingEntityFetcher{entities: []model.Entity{activeTask}}},
		{"invalid_exec_edges", "invalid_exec_edges", checkInvalidExecEdges, failingEntityFetcher{entities: []model.Entity{activePhase}}},
		{"task_scope", "task_scope", checkTaskScope, failingEntityFetcher{entities: []model.Entity{activeTask}}},
		{"mapping_consistency", "mapping_consistency", checkMappingConsistency, failingEntityFetcher{entities: []model.Entity{activePhase}}},
		{"invalid_mapping_edges", "invalid_mapping_edges", checkInvalidMappingEdges, failingEntityFetcher{entities: []model.Entity{activePhase}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := tt.run(failingRelationFetcher{}, tt.ef)

			if len(issues) == 0 {
				t.Fatalf("check %s reported no issues on a storage failure", tt.check)
			}
			for _, issue := range issues {
				if issue.Check != tt.check {
					t.Errorf("issue attributed to check %q, want %q", issue.Check, tt.check)
				}
				if issue.Severity != SeverityHigh {
					t.Errorf("storage failure reported at severity %q, want high", issue.Severity)
				}
			}
			if !mentionsStorageError(issues) {
				t.Errorf("no issue mentions the underlying error; got %+v", issues)
			}
		})
	}
}

// A check that cannot even list its subjects must report that, not return clean.
func TestChecksReportListFailure(t *testing.T) {
	tests := []struct {
		name  string
		check string
		run   func(rf RelationFetcher, ef EntityFetcher) []ValidationIssue
	}{
		{"orphans", "orphans", checkOrphans},
		{"coverage", "coverage", checkCoverage},
		{"cycles", "cycles", checkCycles},
		{"conflicts", "conflicts", checkConflicts},
		{"invalid_edges", "invalid_edges", checkInvalidEdges},
		{"superseded_refs", "superseded_refs", checkSupersededRefs},
		{"unresolved", "unresolved", checkUnresolved},
		{"phase_order", "phase_order", checkPhaseOrder},
		{"orphan_phases", "orphan_phases", checkOrphanPhases},
		{"exec_cycles", "exec_cycles", checkExecCycles},
		{"orphan_changes", "orphan_changes", checkOrphanChanges},
		{"task_graph", "task_graph", checkTaskGraph},
		{"invalid_exec_edges", "invalid_exec_edges", checkInvalidExecEdges},
		{"task_scope", "task_scope", checkTaskScope},
		{"mapping_consistency", "mapping_consistency", checkMappingConsistency},
		{"invalid_mapping_edges", "invalid_mapping_edges", checkInvalidMappingEdges},
		{"delivery_completeness", "delivery_completeness", checkDeliveryCompleteness},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := tt.run(failingRelationFetcher{}, listFailingEntityFetcher{})

			if len(issues) == 0 {
				t.Fatalf("check %s reported no issues when listing failed", tt.check)
			}
			if !mentionsStorageError(issues) {
				t.Errorf("no issue mentions the underlying error; got %+v", issues)
			}
		})
	}
}

func TestSingleActivePlanReportsListFailure(t *testing.T) {
	issues := checkSingleActivePlan(listFailingEntityFetcher{})
	if len(issues) == 0 {
		t.Fatal("checkSingleActivePlan reported no issues when listing failed")
	}
	if !mentionsStorageError(issues) {
		t.Errorf("no issue mentions the underlying error; got %+v", issues)
	}
}

// A missing entity is a legitimate skip, not a storage failure, so checks stay
// quiet about it.
func TestNotFoundIsNotReportedAsFailure(t *testing.T) {
	ef := newMockEntityFetcher(model.Entity{
		ID: "REQ-001", Type: model.EntityTypeRequirement,
		Layer: model.LayerArch, Status: model.EntityStatusActive,
		Metadata: []byte(`{}`),
	})
	// REQ-999 does not exist, so resolving the relation target yields NotFound.
	rf := newMockRelationFetcher(model.Relation{
		FromID: "REQ-001", ToID: "REQ-999",
		Type: model.RelationConflictsWith, Layer: model.LayerArch,
	})

	for _, issue := range checkConflicts(rf, ef) {
		if strings.Contains(issue.Message, "could not read") {
			t.Errorf("NotFound reported as a read failure: %q", issue.Message)
		}
	}
}

func mentionsStorageError(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Message, errStorage.Error()) {
			return true
		}
	}
	return false
}

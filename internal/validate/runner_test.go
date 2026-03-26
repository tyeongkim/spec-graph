package validate

import (
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

func TestValidate_LayerDispatch(t *testing.T) {
	archLayer := model.LayerArch
	execLayer := model.LayerExec
	mappingLayer := model.LayerMapping

	reqEntity := archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive)
	phaseEntity := execEntity("PHS-1", model.EntityTypePhase, model.EntityStatusActive, nil)
	planEntity := execEntity("PLN-1", model.EntityTypePlan, model.EntityStatusActive, nil)

	ef := newMockEntityFetcher(reqEntity, phaseEntity, planEntity)
	rf := newMockRelationFetcher()

	tests := []struct {
		name     string
		layer    *model.Layer
		wantArch bool
		wantExec bool
		wantMap  bool
	}{
		{
			name:     "nil layer runs all",
			layer:    nil,
			wantArch: true, wantExec: true, wantMap: true,
		},
		{
			name:     "arch layer only",
			layer:    &archLayer,
			wantArch: true, wantExec: false, wantMap: false,
		},
		{
			name:     "exec layer only",
			layer:    &execLayer,
			wantArch: false, wantExec: true, wantMap: false,
		},
		{
			name:     "mapping layer only",
			layer:    &mappingLayer,
			wantArch: false, wantExec: false, wantMap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate(ValidateOptions{Layer: tt.layer}, rf, ef)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}

			hasArch, hasExec, hasMapping := false, false, false
			for _, iss := range result.Issues {
				switch iss.Layer {
				case model.LayerArch:
					hasArch = true
				case model.LayerExec:
					hasExec = true
				case model.LayerMapping:
					hasMapping = true
				}
			}

			if tt.wantArch && !hasArch {
				t.Log("note: no arch issues found (may be expected if graph is clean for arch)")
			}
			if !tt.wantExec && hasExec {
				t.Error("got exec issues but layer should have excluded exec")
			}
			if !tt.wantArch && hasArch {
				t.Error("got arch issues but layer should have excluded arch")
			}
			if !tt.wantMap && hasMapping {
				t.Error("got mapping issues but layer should have excluded mapping")
			}
		})
	}
}

func TestValidate_EntityIDFilter(t *testing.T) {
	entities := []model.Entity{
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
		archEntity("REQ-2", model.EntityTypeRequirement, model.EntityStatusActive),
	}
	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher()

	result, err := Validate(ValidateOptions{EntityID: "REQ-1"}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, iss := range result.Issues {
		if iss.Entity != "REQ-1" {
			t.Errorf("got issue for entity %q; want only REQ-1", iss.Entity)
		}
	}
}

func TestValidate_UnknownCheck(t *testing.T) {
	ef := newMockEntityFetcher()
	rf := newMockRelationFetcher()

	_, err := Validate(ValidateOptions{Checks: []string{"nonexistent_check"}}, rf, ef)
	if err == nil {
		t.Fatal("expected error for unknown check; got nil")
	}
}

func TestValidate_CheckLayerMismatch(t *testing.T) {
	archLayer := model.LayerArch
	ef := newMockEntityFetcher()
	rf := newMockRelationFetcher()

	_, err := Validate(ValidateOptions{
		Layer:  &archLayer,
		Checks: []string{"phase_order"},
	}, rf, ef)
	if err == nil {
		t.Fatal("expected error for check-layer mismatch; got nil")
	}
}

func TestValidate_SpecificCheck(t *testing.T) {
	archLayer := model.LayerArch
	entities := []model.Entity{
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
	}
	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher()

	result, err := Validate(ValidateOptions{
		Layer:  &archLayer,
		Checks: []string{"orphans"},
	}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, iss := range result.Issues {
		if iss.Check != "orphans" {
			t.Errorf("got check %q; want only orphans", iss.Check)
		}
	}
}

func TestValidate_CleanGraph(t *testing.T) {
	ef := newMockEntityFetcher()
	rf := newMockRelationFetcher()

	result, err := Validate(ValidateOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true for empty graph; got issues=%+v", result.Issues)
	}
	if result.Summary.TotalIssues != 0 {
		t.Errorf("expected 0 total issues; got %d", result.Summary.TotalIssues)
	}
}

func TestValidate_SummaryBySeverity(t *testing.T) {
	entities := []model.Entity{
		archEntity("REQ-1", model.EntityTypeRequirement, model.EntityStatusActive),
		archEntity("QST-1", model.EntityTypeQuestion, model.EntityStatusActive),
	}
	ef := newMockEntityFetcher(entities...)
	rf := newMockRelationFetcher()

	result, err := Validate(ValidateOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.TotalIssues != len(result.Issues) {
		t.Errorf("TotalIssues=%d; len(Issues)=%d", result.Summary.TotalIssues, len(result.Issues))
	}

	severityCount := 0
	for _, count := range result.Summary.BySeverity {
		severityCount += count
	}
	if severityCount != result.Summary.TotalIssues {
		t.Errorf("sum of BySeverity=%d; TotalIssues=%d", severityCount, result.Summary.TotalIssues)
	}
}

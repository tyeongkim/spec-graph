package gate

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

type readFailureEntityFetcher struct {
	validate.EntityFetcher
	err error
}

func (f readFailureEntityFetcher) Get(string) (model.Entity, error) { return model.Entity{}, f.err }

func TestLifecycleReadFailuresBlockCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(validate.EntityFetcher) []validate.ValidationIssue
	}{
		{"phase children", func(ef validate.EntityFetcher) []validate.ValidationIssue {
			return unresolvedChildren("phase_tasks", "PHS-1", []string{"TSK-1"}, model.EntityTypeTask, ef)
		}},
		{"plan children", func(ef validate.EntityFetcher) []validate.ValidationIssue {
			return unresolvedChildren("plan_phases", "PLN-1", []string{"PHS-1"}, model.EntityTypePhase, ef)
		}},
		{"task deliveries", func(ef validate.EntityFetcher) []validate.ValidationIssue {
			return missingTaskDeliveries("TSK-1", []model.Relation{rel("TSK-1", "REQ-1", model.RelationCovers)}, ef)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readErr := errors.New("index is corrupt")
			issues := tc.run(readFailureEntityFetcher{err: readErr})
			if len(issues) != 1 || issues[0].Severity != validate.SeverityHigh || !strings.Contains(issues[0].Message, readErr.Error()) {
				t.Fatalf("issues=%+v; want blocking read failure", issues)
			}
			missingErr := fmt.Errorf("wrapped: %w", &model.ErrEntityNotFound{ID: "missing"})
			if issues := tc.run(readFailureEntityFetcher{err: missingErr}); len(issues) != 0 {
				t.Fatalf("missing entity should be tolerated: %+v", issues)
			}
		})
	}
}

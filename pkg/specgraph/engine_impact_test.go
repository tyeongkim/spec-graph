package specgraph_test

import (
	"context"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestImpact(t *testing.T) {
	t.Parallel()

	// seedImpact creates API-001 implements REQ-001 (an arch-layer,
	// bidirectional relation), so an impact run from API-001 reaches REQ-001.
	seedImpact := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		return eng, ctx
	}

	t.Run("empty sources", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.Impact(ctx, specgraph.ImpactRequest{Sources: nil})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("valid source returns affected with scores", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedImpact(t)

		res, err := eng.Impact(ctx, specgraph.ImpactRequest{Sources: []string{"API-001"}})
		if err != nil {
			t.Fatalf("Impact: %v", err)
		}
		if len(res.Sources) != 1 || res.Sources[0] != "API-001" {
			t.Errorf("Sources = %v, want [API-001]", res.Sources)
		}
		if len(res.Affected) != 1 {
			t.Fatalf("len(Affected) = %d, want 1", len(res.Affected))
		}
		aff := res.Affected[0]
		if aff.ID != "REQ-001" {
			t.Errorf("Affected[0].ID = %q, want %q", aff.ID, "REQ-001")
		}
		if aff.Overall == "" {
			t.Error("Affected[0].Overall is empty, want a severity")
		}
		if res.Summary.Total != 1 {
			t.Errorf("Summary.Total = %d, want 1", res.Summary.Total)
		}
	})

	t.Run("layer filter restricts results", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedImpact(t)

		// implements is an arch-layer relation; an "exec" layer filter prunes
		// the traversal so no entities are reached.
		res, err := eng.Impact(ctx, specgraph.ImpactRequest{Sources: []string{"API-001"}, Layer: "exec"})
		if err != nil {
			t.Fatalf("Impact exec: %v", err)
		}
		if len(res.Affected) != 0 {
			t.Errorf("exec len(Affected) = %d, want 0", len(res.Affected))
		}

		res, err = eng.Impact(ctx, specgraph.ImpactRequest{Sources: []string{"API-001"}, Layer: "arch"})
		if err != nil {
			t.Fatalf("Impact arch: %v", err)
		}
		if len(res.Affected) != 1 {
			t.Errorf("arch len(Affected) = %d, want 1", len(res.Affected))
		}
	})
}

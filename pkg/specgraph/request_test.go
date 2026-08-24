package specgraph_test

import (
	"context"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// Layer, severity, and dimension rules are enforced here rather than in each
// transport, so the CLI, RPC, and MCP surfaces cannot disagree. RPC in
// particular used to pass these fields straight through, accepting values the
// other two rejected.
func TestRequestFieldValidationIsEnforcedByEngine(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()

	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "Req",
	}); err != nil {
		t.Fatalf("create REQ-001: %v", err)
	}
	if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{
		Type: "phase", ID: "PHS-001", Title: "Phase",
	}); err != nil {
		t.Fatalf("create PHS-001: %v", err)
	}

	// Values a transport might forward unchecked: wrong case, padded, or unknown.
	badLayers := []string{"Arch", "arch ", "bogus"}

	t.Run("query scope rejects bad layer", func(t *testing.T) {
		for _, layer := range badLayers {
			_, err := eng.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: "PHS-001", Layer: layer})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
		}
	})

	t.Run("query path rejects bad layer", func(t *testing.T) {
		for _, layer := range badLayers {
			_, err := eng.QueryPath(ctx, specgraph.QueryPathRequest{
				FromID: "PHS-001", ToID: "REQ-001", Layer: layer,
			})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
		}
	})

	t.Run("impact rejects bad layer", func(t *testing.T) {
		for _, layer := range badLayers {
			_, err := eng.Impact(ctx, specgraph.ImpactRequest{Sources: []string{"REQ-001"}, Layer: layer})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
		}
	})

	t.Run("export rejects bad layer", func(t *testing.T) {
		for _, layer := range badLayers {
			_, err := eng.Export(ctx, specgraph.ExportRequest{Format: "dot", Layer: layer})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
		}
	})

	t.Run("validate rejects bad layer", func(t *testing.T) {
		for _, layer := range badLayers {
			_, err := eng.Validate(ctx, specgraph.ValidateRequest{Layer: layer})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
		}
	})

	t.Run("impact rejects bad severity and dimension", func(t *testing.T) {
		_, err := eng.Impact(ctx, specgraph.ImpactRequest{
			Sources: []string{"REQ-001"}, MinSeverity: "bogus",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)

		_, err = eng.Impact(ctx, specgraph.ImpactRequest{
			Sources: []string{"REQ-001"}, Dimension: "bogus",
		})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	// "all" and "" both mean every layer, and must not be treated as invalid.
	t.Run("accepts all and empty layer", func(t *testing.T) {
		for _, layer := range []string{"", "all", "arch", "exec", "mapping"} {
			if _, err := eng.Validate(ctx, specgraph.ValidateRequest{Layer: layer}); err != nil {
				t.Errorf("Validate rejected layer %q: %v", layer, err)
			}
		}
	})
}

// Every operation now honors its context at entry, so a cancelled caller does
// not take locks or touch the index. Previously ctx was discarded outright.
func TestOperationsHonorCancelledContext(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	setup := context.Background()

	if _, err := eng.CreateEntity(setup, specgraph.CreateEntityRequest{
		Type: "requirement", ID: "REQ-001", Title: "Req",
	}); err != nil {
		t.Fatalf("create REQ-001: %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("reads", func(t *testing.T) {
		if _, err := eng.GetEntity(cancelled, "REQ-001"); err == nil {
			t.Error("GetEntity accepted a cancelled context")
		}
		if _, err := eng.Validate(cancelled, specgraph.ValidateRequest{}); err == nil {
			t.Error("Validate accepted a cancelled context")
		}
		if _, err := eng.Export(cancelled, specgraph.ExportRequest{Format: "dot"}); err == nil {
			t.Error("Export accepted a cancelled context")
		}
		if _, err := eng.Impact(cancelled, specgraph.ImpactRequest{Sources: []string{"REQ-001"}}); err == nil {
			t.Error("Impact accepted a cancelled context")
		}
	})

	t.Run("writes leave the graph unchanged", func(t *testing.T) {
		_, err := eng.CreateEntity(cancelled, specgraph.CreateEntityRequest{
			Type: "requirement", ID: "REQ-002", Title: "Should not exist",
		})
		if err == nil {
			t.Fatal("CreateEntity accepted a cancelled context")
		}

		if _, err := eng.GetEntity(setup, "REQ-002"); err == nil {
			t.Error("cancelled CreateEntity still wrote REQ-002")
		}
	})
}

package specgraph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestSnapshotRejectsReadsAfterCallback(t *testing.T) {
	eng := openSnapshotEngine(t)
	for _, exit := range []string{"return", "error", "panic"} {
		t.Run(exit, func(t *testing.T) {
			var retained *specgraph.Snapshot
			var copied specgraph.Snapshot
			func() {
				defer func() {
					if recovered := recover(); recovered != nil && exit != "panic" {
						panic(recovered)
					}
				}()
				_ = eng.Read(context.Background(), func(s *specgraph.Snapshot) error {
					retained = s
					copied = *s
					switch exit {
					case "error":
						return errors.New("callback failed")
					case "panic":
						panic("callback panicked")
					}
					return nil
				})
			}()
			checks := []func() error{
				func() error { _, _, err := copied.ListEntities(specgraph.ListEntitiesRequest{}); return err },
				func() error { _, _, err := retained.ListEntities(specgraph.ListEntitiesRequest{}); return err },
				func() error { _, err := retained.PhaseContext("PHS-001"); return err },
				func() error { _, err := retained.Validate(specgraph.ValidateRequest{}); return err },
				func() error { _, err := retained.Impact(specgraph.ImpactRequest{}); return err },
				func() error { _, err := retained.QueryNeighbors(specgraph.QueryNeighborsRequest{}); return err },
			}
			for i, check := range checks {
				var apiError *specgraph.Error
				if err := check(); !errors.As(err, &apiError) || apiError.Code != specgraph.CodeInvalidState {
					t.Errorf("method %d after callback = %v; want invalid_state", i, err)
				}
			}
		})
	}
}

func TestIndexReadsPreserveForcedCompletionAudit(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	setupTaskLifecycle(t, engine, "TSK-001", true)
	resolved := "resolved"
	if _, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID: "PHS-001", Status: &resolved, Force: true, Reason: "Accept unfinished task",
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := engine.GetEntity(context.Background(), "PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	listed, _, err := engine.ListEntities(context.Background(), specgraph.ListEntitiesRequest{Type: "phase"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed phases = %d; want 1", len(listed))
	}
	if !listed[0].CompletionForced || listed[0].CompletionReason != stored.CompletionReason {
		t.Fatalf("index audit = (%t, %q), source audit = (%t, %q)", listed[0].CompletionForced, listed[0].CompletionReason, stored.CompletionForced, stored.CompletionReason)
	}
	context, err := engine.PhaseContext(context.Background(), "PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	if !context.Phase.CompletionForced || context.Phase.CompletionReason != stored.CompletionReason {
		t.Fatalf("phase context lost completion audit: %+v", context.Phase)
	}
}

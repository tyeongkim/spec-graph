package specgraph_test

import (
	"context"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestQueryScope(t *testing.T) {
	t.Parallel()

	// seedScope creates a phase that covers a requirement and delivers an
	// interface, returning the engine and ctx.
	seedScope := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()

		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "plan", ID: "PLN-001", Title: "Plan"}); err != nil {
			t.Fatalf("create PLN-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "phase", ID: "PHS-001", Title: "Phase"}); err != nil {
			t.Fatalf("create PHS-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Req"}); err != nil {
			t.Fatalf("create REQ-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "interface", ID: "API-001", Title: "Api"}); err != nil {
			t.Fatalf("create API-001: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"}); err != nil {
			t.Fatalf("AddRelation belongs_to: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-001", To: "REQ-001", Type: "covers"}); err != nil {
			t.Fatalf("AddRelation covers: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "PHS-001", To: "API-001", Type: "delivers"}); err != nil {
			t.Fatalf("AddRelation delivers: %v", err)
		}
		return eng, ctx
	}

	t.Run("empty phase id", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: ""})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("returns covered and delivered entities", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedScope(t)

		res, err := eng.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: "PHS-001"})
		if err != nil {
			t.Fatalf("QueryScope: %v", err)
		}
		if res.PhaseID != "PHS-001" {
			t.Errorf("PhaseID = %q, want %q", res.PhaseID, "PHS-001")
		}
		if len(res.Entities) != 2 {
			t.Fatalf("len(Entities) = %d, want 2", len(res.Entities))
		}
		if len(res.Relations) != 2 {
			t.Errorf("len(Relations) = %d, want 2", len(res.Relations))
		}
		ids := map[string]bool{}
		for _, e := range res.Entities {
			ids[e.ID] = true
		}
		if !ids["REQ-001"] || !ids["API-001"] {
			t.Errorf("expected REQ-001 and API-001 in scope, got %v", ids)
		}
	})

	t.Run("layer filter restricts results", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedScope(t)

		// REQ-001 and API-001 are both arch-layer entities, so an "arch" filter
		// keeps both while an "exec" filter drops both.
		res, err := eng.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: "PHS-001", Layer: "arch"})
		if err != nil {
			t.Fatalf("QueryScope arch: %v", err)
		}
		if len(res.Entities) != 2 {
			t.Errorf("arch len(Entities) = %d, want 2", len(res.Entities))
		}

		res, err = eng.QueryScope(ctx, specgraph.QueryScopeRequest{PhaseID: "PHS-001", Layer: "exec"})
		if err != nil {
			t.Fatalf("QueryScope exec: %v", err)
		}
		if len(res.Entities) != 0 {
			t.Errorf("exec len(Entities) = %d, want 0", len(res.Entities))
		}
	})
}

func TestQueryNeighbors(t *testing.T) {
	t.Parallel()

	// seedNeighbors creates API-001 implements REQ-001.
	seedNeighbors := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		return eng, ctx
	}

	t.Run("empty entity id", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.QueryNeighbors(ctx, specgraph.QueryNeighborsRequest{EntityID: ""})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("depth 0 returns only center", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedNeighbors(t)

		res, err := eng.QueryNeighbors(ctx, specgraph.QueryNeighborsRequest{EntityID: "API-001", Depth: 0})
		if err != nil {
			t.Fatalf("QueryNeighbors: %v", err)
		}
		if res.Center != "API-001" {
			t.Errorf("Center = %q, want %q", res.Center, "API-001")
		}
		if len(res.Entities) != 1 {
			t.Fatalf("len(Entities) = %d, want 1", len(res.Entities))
		}
		if res.Entities[0].Entity.ID != "API-001" {
			t.Errorf("center entity = %q, want %q", res.Entities[0].Entity.ID, "API-001")
		}
	})

	t.Run("depth 1 returns center and direct neighbors", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedNeighbors(t)

		res, err := eng.QueryNeighbors(ctx, specgraph.QueryNeighborsRequest{EntityID: "API-001", Depth: 1})
		if err != nil {
			t.Fatalf("QueryNeighbors: %v", err)
		}
		if len(res.Entities) != 2 {
			t.Fatalf("len(Entities) = %d, want 2", len(res.Entities))
		}
		ids := map[string]int{}
		for _, ne := range res.Entities {
			ids[ne.Entity.ID] = ne.Depth
		}
		if ids["API-001"] != 0 {
			t.Errorf("API-001 depth = %d, want 0", ids["API-001"])
		}
		if ids["REQ-001"] != 1 {
			t.Errorf("REQ-001 depth = %d, want 1", ids["REQ-001"])
		}
	})
}

func TestQueryPath(t *testing.T) {
	t.Parallel()

	// seedPath creates API-001 implements REQ-001, plus an isolated DEC-001.
	seedPath := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		setupRelationTestEntities(t, eng)
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "decision", ID: "DEC-001", Title: "Isolated"}); err != nil {
			t.Fatalf("create DEC-001: %v", err)
		}
		if _, err := eng.AddRelation(ctx, specgraph.AddRelationRequest{From: "API-001", To: "REQ-001", Type: "implements"}); err != nil {
			t.Fatalf("AddRelation implements: %v", err)
		}
		return eng, ctx
	}

	t.Run("missing ids", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		cases := []struct {
			name string
			req  specgraph.QueryPathRequest
		}{
			{name: "empty from", req: specgraph.QueryPathRequest{FromID: "", ToID: "REQ-001"}},
			{name: "empty to", req: specgraph.QueryPathRequest{FromID: "API-001", ToID: ""}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := eng.QueryPath(ctx, tc.req)
				assertErrorCode(t, err, specgraph.CodeInvalidInput)
			})
		}
	})

	t.Run("path exists", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedPath(t)

		res, err := eng.QueryPath(ctx, specgraph.QueryPathRequest{FromID: "API-001", ToID: "REQ-001"})
		if err != nil {
			t.Fatalf("QueryPath: %v", err)
		}
		if !res.Found {
			t.Fatal("Found = false, want true")
		}
		if len(res.Path) != 2 {
			t.Fatalf("len(Path) = %d, want 2", len(res.Path))
		}
		if res.Path[0].EntityID != "API-001" {
			t.Errorf("Path[0] = %q, want %q", res.Path[0].EntityID, "API-001")
		}
		if res.Path[len(res.Path)-1].EntityID != "REQ-001" {
			t.Errorf("Path[last] = %q, want %q", res.Path[len(res.Path)-1].EntityID, "REQ-001")
		}
	})

	t.Run("no path", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedPath(t)

		res, err := eng.QueryPath(ctx, specgraph.QueryPathRequest{FromID: "API-001", ToID: "DEC-001"})
		if err != nil {
			t.Fatalf("QueryPath: %v", err)
		}
		if res.Found {
			t.Error("Found = true, want false")
		}
		if len(res.Path) != 0 {
			t.Errorf("len(Path) = %d, want 0", len(res.Path))
		}
	})
}

func TestQueryUnresolved(t *testing.T) {
	t.Parallel()

	// seedUnresolved creates one draft entity of each unresolved type:
	// question, assumption, and risk.
	seedUnresolved := func(t *testing.T) (*specgraph.Engine, context.Context) {
		t.Helper()
		eng := openTestEngine(t)
		ctx := context.Background()
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "question", ID: "QST-001", Title: "Question"}); err != nil {
			t.Fatalf("create QST-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "assumption", ID: "ASM-001", Title: "Assumption"}); err != nil {
			t.Fatalf("create ASM-001: %v", err)
		}
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "risk", ID: "RSK-001", Title: "Risk"}); err != nil {
			t.Fatalf("create RSK-001: %v", err)
		}
		return eng, ctx
	}

	t.Run("no type filter returns all unresolved", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedUnresolved(t)

		res, err := eng.QueryUnresolved(ctx, specgraph.QueryUnresolvedRequest{})
		if err != nil {
			t.Fatalf("QueryUnresolved: %v", err)
		}
		if res.Count != 3 {
			t.Errorf("Count = %d, want 3", res.Count)
		}
		if len(res.Entities) != 3 {
			t.Errorf("len(Entities) = %d, want 3", len(res.Entities))
		}
	})

	t.Run("type filter returns only that type", func(t *testing.T) {
		t.Parallel()
		eng, ctx := seedUnresolved(t)

		res, err := eng.QueryUnresolved(ctx, specgraph.QueryUnresolvedRequest{Type: "question"})
		if err != nil {
			t.Fatalf("QueryUnresolved: %v", err)
		}
		if res.Count != 1 {
			t.Fatalf("Count = %d, want 1", res.Count)
		}
		if string(res.Entities[0].Type) != "question" {
			t.Errorf("Type = %q, want %q", res.Entities[0].Type, "question")
		}
	})
}

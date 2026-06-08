package specgraph_test

import (
	"context"
	"testing"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("no options runs all checks", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Req"}); err != nil {
			t.Fatalf("create REQ-001: %v", err)
		}

		res, err := eng.Validate(ctx, specgraph.ValidateRequest{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if res == nil {
			t.Fatal("Validate returned nil result")
		}
		if res.Summary.TotalIssues != len(res.Issues) {
			t.Errorf("Summary.TotalIssues = %d, want %d", res.Summary.TotalIssues, len(res.Issues))
		}
	})

	t.Run("invalid layer", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()

		_, err := eng.Validate(ctx, specgraph.ValidateRequest{Layer: "not_a_layer"})
		assertErrorCode(t, err, specgraph.CodeInvalidInput)
	})

	t.Run("specific layer filters checks", func(t *testing.T) {
		t.Parallel()
		eng := openTestEngine(t)
		ctx := context.Background()
		if _, err := eng.CreateEntity(ctx, specgraph.CreateEntityRequest{Type: "requirement", ID: "REQ-001", Title: "Req"}); err != nil {
			t.Fatalf("create REQ-001: %v", err)
		}

		res, err := eng.Validate(ctx, specgraph.ValidateRequest{Layer: "arch"})
		if err != nil {
			t.Fatalf("Validate arch: %v", err)
		}
		if res == nil {
			t.Fatal("Validate returned nil result")
		}
		for _, issue := range res.Issues {
			if string(issue.Layer) != "arch" {
				t.Errorf("issue layer = %q, want arch", issue.Layer)
			}
		}
	})
}

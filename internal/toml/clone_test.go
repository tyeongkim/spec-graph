package spectoml

import (
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestEntityFileCloneDeepCopiesNestedValues(t *testing.T) {
	original := &EntityFile{
		ID: "REQ-001",
		Metadata: map[string]any{
			"nested": map[string]any{"value": "original"},
			"items":  []any{"original", map[string]any{"value": "original"}},
		},
		Relations: []RelationEntry{
			{
				To:   "REQ-002",
				Type: model.RelationDependsOn,
				Metadata: map[string]any{
					"nested": map[string]any{"value": "original"},
					"items":  []any{"original", map[string]any{"value": "original"}},
				},
			},
		},
	}

	clone := original.Clone()
	clone.Metadata["nested"].(map[string]any)["value"] = "changed"
	clone.Metadata["items"].([]any)[0] = "changed"
	clone.Metadata["items"].([]any)[1].(map[string]any)["value"] = "changed"
	clone.Relations[0].To = "REQ-003"
	clone.Relations[0].Metadata["nested"].(map[string]any)["value"] = "changed"
	clone.Relations[0].Metadata["items"].([]any)[0] = "changed"
	clone.Relations[0].Metadata["items"].([]any)[1].(map[string]any)["value"] = "changed"

	if original.Metadata["nested"].(map[string]any)["value"] != "original" {
		t.Errorf("original nested metadata = %v, want %q", original.Metadata["nested"], "original")
	}
	if original.Metadata["items"].([]any)[0] != "original" {
		t.Errorf("original metadata items[0] = %v, want %q", original.Metadata["items"].([]any)[0], "original")
	}
	if original.Metadata["items"].([]any)[1].(map[string]any)["value"] != "original" {
		t.Errorf("original metadata items[1] = %v, want %q", original.Metadata["items"].([]any)[1], "original")
	}
	if original.Relations[0].To != "REQ-002" {
		t.Errorf("original relation To = %q, want %q", original.Relations[0].To, "REQ-002")
	}
	if original.Relations[0].Metadata["nested"].(map[string]any)["value"] != "original" {
		t.Errorf("original relation nested metadata = %v, want %q", original.Relations[0].Metadata["nested"], "original")
	}
	if original.Relations[0].Metadata["items"].([]any)[0] != "original" {
		t.Errorf("original relation metadata items[0] = %v, want %q", original.Relations[0].Metadata["items"].([]any)[0], "original")
	}
	if original.Relations[0].Metadata["items"].([]any)[1].(map[string]any)["value"] != "original" {
		t.Errorf("original relation metadata items[1] = %v, want %q", original.Relations[0].Metadata["items"].([]any)[1], "original")
	}

	nilClone := (&EntityFile{}).Clone()
	if nilClone.Metadata != nil {
		t.Errorf("nil Metadata cloned as %v, want nil", nilClone.Metadata)
	}
	if nilClone.Relations != nil {
		t.Errorf("nil Relations cloned as %v, want nil", nilClone.Relations)
	}
}

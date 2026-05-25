package spectoml

import (
	"encoding/json"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestMarshalEntityFile_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		input    EntityFile
		expected string
	}{
		{
			name: "full entity with relations",
			input: EntityFile{
				Schema:      1,
				ID:          "REQ-001",
				Type:        model.EntityTypeRequirement,
				Title:       "User authentication",
				Description: "Users must authenticate via OAuth2",
				Status:      model.EntityStatusActive,
				Metadata: map[string]any{
					"priority": "must",
					"kind":     "non_functional",
				},
				Relations: []RelationEntry{
					{To: "DEC-001", Type: model.RelationConstrainedBy, Weight: 0.8},
					{To: "ACT-001", Type: model.RelationHasCriterion},
				},
			},
			expected: `schema = 1
id = "REQ-001"
type = "requirement"
title = "User authentication"
description = "Users must authenticate via OAuth2"
status = "active"

[metadata]
kind = "non_functional"
priority = "must"

[[relations]]
to = "DEC-001"
type = "constrained_by"
weight = 0.8

[[relations]]
to = "ACT-001"
type = "has_criterion"
`,
		},
		{
			name: "entity without description",
			input: EntityFile{
				Schema: 1,
				ID:     "DEC-001",
				Type:   model.EntityTypeDecision,
				Title:  "Adopt JWT",
				Status: model.EntityStatusDraft,
			},
			expected: `schema = 1
id = "DEC-001"
type = "decision"
title = "Adopt JWT"
status = "draft"
`,
		},
		{
			name: "entity with relation metadata",
			input: EntityFile{
				Schema: 1,
				ID:     "PHS-001",
				Type:   model.EntityTypePhase,
				Title:  "Phase 1",
				Status: model.EntityStatusActive,
				Relations: []RelationEntry{
					{
						To:       "REQ-001",
						Type:     model.RelationCovers,
						Metadata: map[string]any{"scope": "partial"},
					},
				},
			},
			expected: `schema = 1
id = "PHS-001"
type = "phase"
title = "Phase 1"
status = "active"

[[relations]]
to = "REQ-001"
type = "covers"
metadata = {scope = "partial"}
`,
		},
		{
			name: "relations sorted by type then to",
			input: EntityFile{
				Schema: 1,
				ID:     "REQ-002",
				Type:   model.EntityTypeRequirement,
				Title:  "Sorting test",
				Status: model.EntityStatusActive,
				Relations: []RelationEntry{
					{To: "DEC-002", Type: model.RelationDependsOn},
					{To: "DEC-001", Type: model.RelationDependsOn},
					{To: "ACT-001", Type: model.RelationConstrainedBy},
				},
			},
			expected: `schema = 1
id = "REQ-002"
type = "requirement"
title = "Sorting test"
status = "active"

[[relations]]
to = "ACT-001"
type = "constrained_by"

[[relations]]
to = "DEC-001"
type = "depends_on"

[[relations]]
to = "DEC-002"
type = "depends_on"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarshalEntityFile(tt.input)
			if got != tt.expected {
				t.Errorf("MarshalEntityFile() mismatch:\ngot:\n%s\nwant:\n%s", got, tt.expected)
			}

			var parsed EntityFile
			if _, err := toml.Decode(got, &parsed); err != nil {
				t.Fatalf("failed to parse canonical output: %v", err)
			}

			if parsed.ID != tt.input.ID {
				t.Errorf("round-trip ID: got %q, want %q", parsed.ID, tt.input.ID)
			}
			if parsed.Type != tt.input.Type {
				t.Errorf("round-trip Type: got %q, want %q", parsed.Type, tt.input.Type)
			}
			if parsed.Title != tt.input.Title {
				t.Errorf("round-trip Title: got %q, want %q", parsed.Title, tt.input.Title)
			}
			if parsed.Description != tt.input.Description {
				t.Errorf("round-trip Description: got %q, want %q", parsed.Description, tt.input.Description)
			}
			if parsed.Status != tt.input.Status {
				t.Errorf("round-trip Status: got %q, want %q", parsed.Status, tt.input.Status)
			}
			if len(parsed.Relations) != len(tt.input.Relations) {
				t.Errorf("round-trip Relations count: got %d, want %d", len(parsed.Relations), len(tt.input.Relations))
			}
		})
	}
}

func TestEntityFileFrom_RoundTrip(t *testing.T) {
	entity := model.Entity{
		ID:          "REQ-001",
		Type:        model.EntityTypeRequirement,
		Layer:       model.LayerArch,
		Title:       "User authentication",
		Description: "Users must authenticate via OAuth2",
		Status:      model.EntityStatusActive,
		Metadata:    json.RawMessage(`{"priority":"must","kind":"non_functional"}`),
	}

	relations := []model.Relation{
		{
			FromID: "REQ-001",
			ToID:   "ACT-001",
			Type:   model.RelationHasCriterion,
			Layer:  model.LayerArch,
			Weight: 1.0,
		},
		{
			FromID: "REQ-001",
			ToID:   "DEC-001",
			Type:   model.RelationConstrainedBy,
			Layer:  model.LayerArch,
			Weight: 0.8,
		},
	}

	ef, err := EntityFileFrom(entity, relations)
	if err != nil {
		t.Fatalf("EntityFileFrom: %v", err)
	}

	output := MarshalEntityFile(ef)

	var parsed EntityFile
	if _, err := toml.Decode(output, &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	gotEntity, err := parsed.ToEntity()
	if err != nil {
		t.Fatalf("ToEntity: %v", err)
	}

	if gotEntity.ID != entity.ID {
		t.Errorf("ID: got %q, want %q", gotEntity.ID, entity.ID)
	}
	if gotEntity.Type != entity.Type {
		t.Errorf("Type: got %q, want %q", gotEntity.Type, entity.Type)
	}
	if gotEntity.Layer != entity.Layer {
		t.Errorf("Layer: got %q, want %q", gotEntity.Layer, entity.Layer)
	}
	if gotEntity.Title != entity.Title {
		t.Errorf("Title: got %q, want %q", gotEntity.Title, entity.Title)
	}
	if gotEntity.Description != entity.Description {
		t.Errorf("Description: got %q, want %q", gotEntity.Description, entity.Description)
	}
	if gotEntity.Status != entity.Status {
		t.Errorf("Status: got %q, want %q", gotEntity.Status, entity.Status)
	}

	gotRelations, err := parsed.ToRelations()
	if err != nil {
		t.Fatalf("ToRelations: %v", err)
	}
	if len(gotRelations) != len(relations) {
		t.Fatalf("Relations count: got %d, want %d", len(gotRelations), len(relations))
	}
}

func TestMarshalEntityFile_Deterministic(t *testing.T) {
	ef := EntityFile{
		Schema: 1,
		ID:     "REQ-001",
		Type:   model.EntityTypeRequirement,
		Title:  "Determinism test",
		Status: model.EntityStatusActive,
		Metadata: map[string]any{
			"z_key": "last",
			"a_key": "first",
			"m_key": "middle",
		},
		Relations: []RelationEntry{
			{To: "DEC-002", Type: model.RelationDependsOn},
			{To: "ACT-001", Type: model.RelationConstrainedBy},
			{To: "DEC-001", Type: model.RelationDependsOn},
		},
	}

	first := MarshalEntityFile(ef)
	for i := 0; i < 100; i++ {
		got := MarshalEntityFile(ef)
		if got != first {
			t.Fatalf("non-deterministic output on iteration %d", i)
		}
	}
}

func TestRelationWeight_DefaultOmitted(t *testing.T) {
	ef := EntityFile{
		Schema: 1,
		ID:     "REQ-001",
		Type:   model.EntityTypeRequirement,
		Title:  "Weight test",
		Status: model.EntityStatusActive,
		Relations: []RelationEntry{
			{To: "DEC-001", Type: model.RelationDependsOn, Weight: 1.0},
			{To: "DEC-002", Type: model.RelationDependsOn, Weight: 0.5},
		},
	}

	got := MarshalEntityFile(ef)

	var parsed EntityFile
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed.Relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(parsed.Relations))
	}

	for _, rel := range parsed.Relations {
		if rel.To == "DEC-001" && rel.Weight != 0 {
			t.Errorf("weight=1.0 should be omitted from output, but parsed as %v", rel.Weight)
		}
		if rel.To == "DEC-002" && rel.Weight != 0.5 {
			t.Errorf("weight=0.5 should be preserved, got %v", rel.Weight)
		}
	}
}

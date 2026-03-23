package graph

import (
	"encoding/json"
	"testing"

	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
)

func TestExportDOT(t *testing.T) {
	tests := []struct {
		name      string
		entities  []model.Entity
		relations []model.Relation
		want      string
	}{
		{
			name:      "empty graph",
			entities:  nil,
			relations: nil,
			want:      "digraph spec_graph {\n}\n",
		},
		{
			name:      "empty slices",
			entities:  []model.Entity{},
			relations: []model.Relation{},
			want:      "digraph spec_graph {\n}\n",
		},
		{
			name: "single node no edges",
			entities: []model.Entity{
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login feature"},
			},
			want: "digraph spec_graph {\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nLogin feature\" shape=box];\n" +
				"}\n",
		},
		{
			name: "nodes and edges sorted deterministically",
			entities: []model.Entity{
				{ID: "API-005", Type: model.EntityTypeInterface, Title: "Auth API"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
				{ID: "DEC-002", Type: model.EntityTypeDecision, Title: "Use JWT"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "DEC-002", Type: model.RelationDependsOn},
				{FromID: "API-005", ToID: "REQ-001", Type: model.RelationImplements},
			},
			want: "digraph spec_graph {\n" +
				"  \"API-005\" [label=\"API-005\\nAuth API\" shape=hexagon];\n" +
				"  \"DEC-002\" [label=\"DEC-002\\nUse JWT\" shape=diamond];\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nLogin\" shape=box];\n" +
				"  \"API-005\" -> \"REQ-001\" [label=\"implements\"];\n" +
				"  \"REQ-001\" -> \"DEC-002\" [label=\"depends_on\"];\n" +
				"}\n",
		},
		{
			name: "special characters in title",
			entities: []model.Entity{
				{ID: "REQ-010", Type: model.EntityTypeRequirement, Title: `He said "hello" & goodbye`},
			},
			want: "digraph spec_graph {\n" +
				"  \"REQ-010\" [label=\"REQ-010\\nHe said \\\"hello\\\" & goodbye\" shape=box];\n" +
				"}\n",
		},
		{
			name: "all entity types have shapes",
			entities: []model.Entity{
				{ID: "ACT-001", Type: model.EntityTypeCriterion, Title: "C"},
				{ID: "API-001", Type: model.EntityTypeInterface, Title: "I"},
				{ID: "ASM-001", Type: model.EntityTypeAssumption, Title: "A"},
				{ID: "DEC-001", Type: model.EntityTypeDecision, Title: "D"},
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "P"},
				{ID: "QST-001", Type: model.EntityTypeQuestion, Title: "Q"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "R"},
				{ID: "RSK-001", Type: model.EntityTypeRisk, Title: "K"},
				{ID: "STT-001", Type: model.EntityTypeState, Title: "S"},
				{ID: "TST-001", Type: model.EntityTypeTest, Title: "T"},
				{ID: "XCT-001", Type: model.EntityTypeCrosscut, Title: "X"},
			},
			want: "digraph spec_graph {\n" +
				"  \"ACT-001\" [label=\"ACT-001\\nC\" shape=cds];\n" +
				"  \"API-001\" [label=\"API-001\\nI\" shape=hexagon];\n" +
				"  \"ASM-001\" [label=\"ASM-001\\nA\" shape=house];\n" +
				"  \"DEC-001\" [label=\"DEC-001\\nD\" shape=diamond];\n" +
				"  \"PHS-001\" [label=\"PHS-001\\nP\" shape=ellipse];\n" +
				"  \"QST-001\" [label=\"QST-001\\nQ\" shape=note];\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nR\" shape=box];\n" +
				"  \"RSK-001\" [label=\"RSK-001\\nK\" shape=triangle];\n" +
				"  \"STT-001\" [label=\"STT-001\\nS\" shape=octagon];\n" +
				"  \"TST-001\" [label=\"TST-001\\nT\" shape=component];\n" +
				"  \"XCT-001\" [label=\"XCT-001\\nX\" shape=parallelogram];\n" +
				"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportDOT(tt.entities, tt.relations)
			if got != tt.want {
				t.Errorf("ExportDOT() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestExportMermaid(t *testing.T) {
	tests := []struct {
		name      string
		entities  []model.Entity
		relations []model.Relation
		want      string
	}{
		{
			name:      "empty graph",
			entities:  nil,
			relations: nil,
			want:      "flowchart LR\n",
		},
		{
			name:      "empty slices",
			entities:  []model.Entity{},
			relations: []model.Relation{},
			want:      "flowchart LR\n",
		},
		{
			name: "single node no edges",
			entities: []model.Entity{
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login feature"},
			},
			want: "flowchart LR\n" +
				"  REQ-001[\"REQ-001: Login feature\"]\n",
		},
		{
			name: "nodes and edges sorted deterministically",
			entities: []model.Entity{
				{ID: "API-005", Type: model.EntityTypeInterface, Title: "Auth API"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
				{ID: "DEC-002", Type: model.EntityTypeDecision, Title: "Use JWT"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "DEC-002", Type: model.RelationDependsOn},
				{FromID: "API-005", ToID: "REQ-001", Type: model.RelationImplements},
			},
			want: "flowchart LR\n" +
				"  API-005{{\"API-005: Auth API\"}}\n" +
				"  DEC-002{\"DEC-002: Use JWT\"}\n" +
				"  REQ-001[\"REQ-001: Login\"]\n" +
				"  API-005 -->|implements| REQ-001\n" +
				"  REQ-001 -->|depends_on| DEC-002\n",
		},
		{
			name: "special characters escaped",
			entities: []model.Entity{
				{ID: "REQ-010", Type: model.EntityTypeRequirement, Title: `Data [array] with "quotes" and |pipes|`},
			},
			want: "flowchart LR\n" +
				"  REQ-010[\"REQ-010: Data &#91;array&#93; with &quot;quotes&quot; and &#124;pipes&#124;\"]\n",
		},
		{
			name: "all entity types have bracket styles",
			entities: []model.Entity{
				{ID: "ACT-001", Type: model.EntityTypeCriterion, Title: "C"},
				{ID: "API-001", Type: model.EntityTypeInterface, Title: "I"},
				{ID: "ASM-001", Type: model.EntityTypeAssumption, Title: "A"},
				{ID: "DEC-001", Type: model.EntityTypeDecision, Title: "D"},
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "P"},
				{ID: "QST-001", Type: model.EntityTypeQuestion, Title: "Q"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "R"},
				{ID: "RSK-001", Type: model.EntityTypeRisk, Title: "K"},
				{ID: "STT-001", Type: model.EntityTypeState, Title: "S"},
				{ID: "TST-001", Type: model.EntityTypeTest, Title: "T"},
				{ID: "XCT-001", Type: model.EntityTypeCrosscut, Title: "X"},
			},
			want: "flowchart LR\n" +
				"  ACT-001((\"ACT-001: C\"))\n" +
				"  API-001{{\"API-001: I\"}}\n" +
				"  ASM-001(\"ASM-001: A\")\n" +
				"  DEC-001{\"DEC-001: D\"}\n" +
				"  PHS-001([\"PHS-001: P\"])\n" +
				"  QST-001>\"QST-001: Q\"]\n" +
				"  REQ-001[\"REQ-001: R\"]\n" +
				"  RSK-001[/\"RSK-001: K\"\\]\n" +
				"  STT-001[[\"STT-001: S\"]]\n" +
				"  TST-001([\"TST-001: T\"])\n" +
				"  XCT-001[/\"XCT-001: X\"/]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportMermaid(tt.entities, tt.relations)
			if got != tt.want {
				t.Errorf("ExportMermaid() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestExportDeterminism(t *testing.T) {
	entities := []model.Entity{
		{ID: "DEC-002", Type: model.EntityTypeDecision, Title: "Use JWT"},
		{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
		{ID: "API-005", Type: model.EntityTypeInterface, Title: "Auth API"},
	}
	relations := []model.Relation{
		{FromID: "REQ-001", ToID: "DEC-002", Type: model.RelationDependsOn},
		{FromID: "API-005", ToID: "REQ-001", Type: model.RelationImplements},
	}

	dotFirst := ExportDOT(entities, relations)
	mermaidFirst := ExportMermaid(entities, relations)

	for i := 0; i < 10; i++ {
		if got := ExportDOT(entities, relations); got != dotFirst {
			t.Fatalf("ExportDOT non-deterministic on iteration %d", i)
		}
		if got := ExportMermaid(entities, relations); got != mermaidFirst {
			t.Fatalf("ExportMermaid non-deterministic on iteration %d", i)
		}
	}
}

func TestExportJSON(t *testing.T) {
	tests := []struct {
		name      string
		entities  []model.Entity
		relations []model.Relation
		want      jsoncontract.ExportJSONResult
	}{
		{
			name:     "empty graph",
			entities: nil,
			want: jsoncontract.ExportJSONResult{
				Entities:  []jsoncontract.ExportJSONEntity{},
				Relations: []jsoncontract.ExportJSONRelation{},
			},
		},
		{
			name: "entities with metadata",
			entities: []model.Entity{
				{ID: "REQ-002", Type: model.EntityTypeRequirement, Title: "Signup", Status: model.EntityStatusDraft, Metadata: json.RawMessage(`{"priority":"high"}`)},
				{ID: "DEC-001", Type: model.EntityTypeDecision, Title: "Use OAuth", Status: model.EntityStatusActive, Metadata: nil},
			},
			relations: []model.Relation{
				{FromID: "REQ-002", ToID: "DEC-001", Type: model.RelationDependsOn, Weight: 0.8},
			},
			want: jsoncontract.ExportJSONResult{
				Entities: []jsoncontract.ExportJSONEntity{
					{ID: "DEC-001", Type: "decision", Title: "Use OAuth", Status: "active", Metadata: map[string]interface{}{}},
					{ID: "REQ-002", Type: "requirement", Title: "Signup", Status: "draft", Metadata: map[string]interface{}{"priority": "high"}},
				},
				Relations: []jsoncontract.ExportJSONRelation{
					{FromID: "REQ-002", ToID: "DEC-001", Type: "depends_on", Weight: 0.8},
				},
			},
		},
		{
			name: "sorted deterministically",
			entities: []model.Entity{
				{ID: "TST-001", Type: model.EntityTypeTest, Title: "T1", Status: model.EntityStatusActive},
				{ID: "API-001", Type: model.EntityTypeInterface, Title: "A1", Status: model.EntityStatusActive},
			},
			relations: []model.Relation{
				{FromID: "TST-001", ToID: "API-001", Type: model.RelationVerifies, Weight: 1.0},
				{FromID: "API-001", ToID: "TST-001", Type: model.RelationDependsOn, Weight: 0.5},
			},
			want: jsoncontract.ExportJSONResult{
				Entities: []jsoncontract.ExportJSONEntity{
					{ID: "API-001", Type: "interface", Title: "A1", Status: "active", Metadata: map[string]interface{}{}},
					{ID: "TST-001", Type: "test", Title: "T1", Status: "active", Metadata: map[string]interface{}{}},
				},
				Relations: []jsoncontract.ExportJSONRelation{
					{FromID: "API-001", ToID: "TST-001", Type: "depends_on", Weight: 0.5},
					{FromID: "TST-001", ToID: "API-001", Type: "verifies", Weight: 1.0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportJSON(tt.entities, tt.relations)

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("ExportJSON() =\n%s\nwant:\n%s", gotJSON, wantJSON)
			}
		})
	}
}

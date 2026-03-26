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
		opts      *ExportOptions
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
				"  \"REQ-001\" [label=\"REQ-001\\nLogin feature\" shape=box style=filled fillcolor=lightblue];\n" +
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
				"  \"API-005\" [label=\"API-005\\nAuth API\" shape=hexagon style=filled fillcolor=lightblue];\n" +
				"  \"DEC-002\" [label=\"DEC-002\\nUse JWT\" shape=diamond style=filled fillcolor=lightblue];\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nLogin\" shape=box style=filled fillcolor=lightblue];\n" +
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
				"  \"REQ-010\" [label=\"REQ-010\\nHe said \\\"hello\\\" & goodbye\" shape=box style=filled fillcolor=lightblue];\n" +
				"}\n",
		},
		{
			name: "all entity types have shapes and layer colors",
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
				"  \"ACT-001\" [label=\"ACT-001\\nC\" shape=cds style=filled fillcolor=lightblue];\n" +
				"  \"API-001\" [label=\"API-001\\nI\" shape=hexagon style=filled fillcolor=lightblue];\n" +
				"  \"ASM-001\" [label=\"ASM-001\\nA\" shape=house style=filled fillcolor=lightblue];\n" +
				"  \"DEC-001\" [label=\"DEC-001\\nD\" shape=diamond style=filled fillcolor=lightblue];\n" +
				"  \"PHS-001\" [label=\"PHS-001\\nP\" shape=ellipse style=filled fillcolor=lightyellow];\n" +
				"  \"QST-001\" [label=\"QST-001\\nQ\" shape=note style=filled fillcolor=lightblue];\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nR\" shape=box style=filled fillcolor=lightblue];\n" +
				"  \"RSK-001\" [label=\"RSK-001\\nK\" shape=triangle style=filled fillcolor=lightblue];\n" +
				"  \"STT-001\" [label=\"STT-001\\nS\" shape=octagon style=filled fillcolor=lightblue];\n" +
				"  \"TST-001\" [label=\"TST-001\\nT\" shape=component style=filled fillcolor=lightblue];\n" +
				"  \"XCT-001\" [label=\"XCT-001\\nX\" shape=parallelogram style=filled fillcolor=lightblue];\n" +
				"}\n",
		},
		{
			name: "mapping relations use dashed style",
			entities: []model.Entity{
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			},
			want: "digraph spec_graph {\n" +
				"  \"PHS-001\" [label=\"PHS-001\\nPhase 1\" shape=ellipse style=filled fillcolor=lightyellow];\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nLogin\" shape=box style=filled fillcolor=lightblue];\n" +
				"  \"REQ-001\" -> \"PHS-001\" [label=\"planned_in\" style=dashed];\n" +
				"}\n",
		},
		{
			name: "layer filter excludes non-matching entities",
			entities: []model.Entity{
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			},
			opts: &ExportOptions{Layer: layerPtr(model.LayerArch)},
			want: "digraph spec_graph {\n" +
				"  \"REQ-001\" [label=\"REQ-001\\nLogin\" shape=box style=filled fillcolor=lightblue];\n" +
				"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportDOT(tt.entities, tt.relations, tt.opts)
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
		opts      *ExportOptions
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
				"  REQ-001[\"REQ-001: Login feature\"]:::arch\n",
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
				"  API-005{{\"API-005: Auth API\"}}:::arch\n" +
				"  DEC-002{\"DEC-002: Use JWT\"}:::arch\n" +
				"  REQ-001[\"REQ-001: Login\"]:::arch\n" +
				"  API-005 -->|implements| REQ-001\n" +
				"  REQ-001 -->|depends_on| DEC-002\n",
		},
		{
			name: "special characters escaped",
			entities: []model.Entity{
				{ID: "REQ-010", Type: model.EntityTypeRequirement, Title: `Data [array] with "quotes" and |pipes|`},
			},
			want: "flowchart LR\n" +
				"  REQ-010[\"REQ-010: Data &#91;array&#93; with &quot;quotes&quot; and &#124;pipes&#124;\"]:::arch\n",
		},
		{
			name: "all entity types have bracket styles and layer classes",
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
				"  ACT-001((\"ACT-001: C\")):::arch\n" +
				"  API-001{{\"API-001: I\"}}:::arch\n" +
				"  ASM-001(\"ASM-001: A\"):::arch\n" +
				"  DEC-001{\"DEC-001: D\"}:::arch\n" +
				"  PHS-001([\"PHS-001: P\"]):::exec\n" +
				"  QST-001>\"QST-001: Q\"]:::arch\n" +
				"  REQ-001[\"REQ-001: R\"]:::arch\n" +
				"  RSK-001[/\"RSK-001: K\"\\]:::arch\n" +
				"  STT-001[[\"STT-001: S\"]]:::arch\n" +
				"  TST-001([\"TST-001: T\"]):::arch\n" +
				"  XCT-001[/\"XCT-001: X\"/]:::arch\n",
		},
		{
			name: "mapping relations use dotted arrows",
			entities: []model.Entity{
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			},
			want: "flowchart LR\n" +
				"  PHS-001([\"PHS-001: Phase 1\"]):::exec\n" +
				"  REQ-001[\"REQ-001: Login\"]:::arch\n" +
				"  REQ-001 -.->|planned_in| PHS-001\n",
		},
		{
			name: "layer filter keeps only exec entities",
			entities: []model.Entity{
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1"},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login"},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			},
			opts: &ExportOptions{Layer: layerPtr(model.LayerExec)},
			want: "flowchart LR\n" +
				"  PHS-001([\"PHS-001: Phase 1\"]):::exec\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportMermaid(tt.entities, tt.relations, tt.opts)
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

	dotFirst := ExportDOT(entities, relations, nil)
	mermaidFirst := ExportMermaid(entities, relations, nil)

	for i := 0; i < 10; i++ {
		if got := ExportDOT(entities, relations, nil); got != dotFirst {
			t.Fatalf("ExportDOT non-deterministic on iteration %d", i)
		}
		if got := ExportMermaid(entities, relations, nil); got != mermaidFirst {
			t.Fatalf("ExportMermaid non-deterministic on iteration %d", i)
		}
	}
}

func TestExportJSON(t *testing.T) {
	tests := []struct {
		name      string
		entities  []model.Entity
		relations []model.Relation
		opts      *ExportOptions
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
			name: "entities with metadata and layer",
			entities: []model.Entity{
				{ID: "REQ-002", Type: model.EntityTypeRequirement, Title: "Signup", Status: model.EntityStatusDraft, Metadata: json.RawMessage(`{"priority":"high"}`)},
				{ID: "DEC-001", Type: model.EntityTypeDecision, Title: "Use OAuth", Status: model.EntityStatusActive, Metadata: nil},
			},
			relations: []model.Relation{
				{FromID: "REQ-002", ToID: "DEC-001", Type: model.RelationDependsOn, Weight: 0.8},
			},
			want: jsoncontract.ExportJSONResult{
				Entities: []jsoncontract.ExportJSONEntity{
					{ID: "DEC-001", Type: "decision", Title: "Use OAuth", Status: "active", Layer: "arch", Metadata: map[string]interface{}{}},
					{ID: "REQ-002", Type: "requirement", Title: "Signup", Status: "draft", Layer: "arch", Metadata: map[string]interface{}{"priority": "high"}},
				},
				Relations: []jsoncontract.ExportJSONRelation{
					{FromID: "REQ-002", ToID: "DEC-001", Type: "depends_on", Layer: "arch", Weight: 0.8},
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
					{ID: "API-001", Type: "interface", Title: "A1", Status: "active", Layer: "arch", Metadata: map[string]interface{}{}},
					{ID: "TST-001", Type: "test", Title: "T1", Status: "active", Layer: "arch", Metadata: map[string]interface{}{}},
				},
				Relations: []jsoncontract.ExportJSONRelation{
					{FromID: "API-001", ToID: "TST-001", Type: "depends_on", Layer: "arch", Weight: 0.5},
					{FromID: "TST-001", ToID: "API-001", Type: "verifies", Layer: "arch", Weight: 1.0},
				},
			},
		},
		{
			name: "layer filter on JSON export",
			entities: []model.Entity{
				{ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1", Status: model.EntityStatusActive},
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Login", Status: model.EntityStatusDraft},
			},
			relations: []model.Relation{
				{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn, Weight: 1.0},
			},
			opts: &ExportOptions{Layer: layerPtr(model.LayerExec)},
			want: jsoncontract.ExportJSONResult{
				Entities: []jsoncontract.ExportJSONEntity{
					{ID: "PHS-001", Type: "phase", Title: "Phase 1", Status: "active", Layer: "exec", Metadata: map[string]interface{}{}},
				},
				Relations: []jsoncontract.ExportJSONRelation{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportJSON(tt.entities, tt.relations, tt.opts)

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("ExportJSON() =\n%s\nwant:\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestFilterByLayer(t *testing.T) {
	entities := []model.Entity{
		{ID: "REQ-001", Type: model.EntityTypeRequirement},
		{ID: "DEC-001", Type: model.EntityTypeDecision},
		{ID: "PHS-001", Type: model.EntityTypePhase},
	}
	relations := []model.Relation{
		{FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn},
		{FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
		{FromID: "PHS-001", ToID: "PHS-001", Type: model.RelationPrecedes},
	}

	t.Run("nil layer returns all", func(t *testing.T) {
		fe, fr := filterByLayer(entities, relations, nil)
		if len(fe) != 3 {
			t.Errorf("got %d entities; want 3", len(fe))
		}
		if len(fr) != 3 {
			t.Errorf("got %d relations; want 3", len(fr))
		}
	})

	t.Run("arch layer filters to arch entities and relations", func(t *testing.T) {
		l := model.LayerArch
		fe, fr := filterByLayer(entities, relations, &l)
		if len(fe) != 2 {
			t.Errorf("got %d entities; want 2", len(fe))
		}
		if len(fr) != 1 {
			t.Errorf("got %d relations; want 1 (depends_on)", len(fr))
		}
		if len(fr) > 0 && fr[0].Type != model.RelationDependsOn {
			t.Errorf("got relation type %s; want depends_on", fr[0].Type)
		}
	})

	t.Run("exec layer filters to exec entities", func(t *testing.T) {
		l := model.LayerExec
		fe, fr := filterByLayer(entities, relations, &l)
		if len(fe) != 1 {
			t.Errorf("got %d entities; want 1", len(fe))
		}
		if fe[0].ID != "PHS-001" {
			t.Errorf("got entity %s; want PHS-001", fe[0].ID)
		}
		if len(fr) != 1 {
			t.Errorf("got %d relations; want 1 (precedes)", len(fr))
		}
	})

	t.Run("mapping relations included when both endpoints match", func(t *testing.T) {
		ents := []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement},
			{ID: "REQ-002", Type: model.EntityTypeRequirement},
		}
		rels := []model.Relation{
			{FromID: "REQ-001", ToID: "REQ-002", Type: model.RelationCovers},
			{FromID: "REQ-001", ToID: "REQ-002", Type: model.RelationDependsOn},
		}
		l := model.LayerArch
		_, fr := filterByLayer(ents, rels, &l)
		if len(fr) != 2 {
			t.Errorf("got %d relations; want 2 (covers + depends_on)", len(fr))
		}
	})
}

func layerPtr(l model.Layer) *model.Layer {
	return &l
}

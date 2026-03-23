package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/taeyeong/spec-graph/internal/model"
)

// dotShape maps entity types to Graphviz DOT node shapes.
var dotShape = map[model.EntityType]string{
	model.EntityTypeRequirement: "box",
	model.EntityTypeDecision:    "diamond",
	model.EntityTypePhase:       "ellipse",
	model.EntityTypeInterface:   "hexagon",
	model.EntityTypeState:       "octagon",
	model.EntityTypeTest:        "component",
	model.EntityTypeCrosscut:    "parallelogram",
	model.EntityTypeQuestion:    "note",
	model.EntityTypeAssumption:  "house",
	model.EntityTypeCriterion:   "cds",
	model.EntityTypeRisk:        "triangle",
}

// ExportDOT renders entities and relations as a Graphviz DOT digraph.
// Output is deterministic: nodes sorted by ID, edges by (from_id, to_id, type).
func ExportDOT(entities []model.Entity, relations []model.Relation) string {
	var b strings.Builder
	b.WriteString("digraph spec_graph {\n")

	sorted := make([]model.Entity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, e := range sorted {
		shape := dotShape[e.Type]
		if shape == "" {
			shape = "box"
		}
		fmt.Fprintf(&b, "  %q [label=%q shape=%s];\n", e.ID, e.ID+"\n"+e.Title, shape)
	}

	sortedRels := make([]model.Relation, len(relations))
	copy(sortedRels, relations)
	sort.Slice(sortedRels, func(i, j int) bool {
		if sortedRels[i].FromID != sortedRels[j].FromID {
			return sortedRels[i].FromID < sortedRels[j].FromID
		}
		if sortedRels[i].ToID != sortedRels[j].ToID {
			return sortedRels[i].ToID < sortedRels[j].ToID
		}
		return sortedRels[i].Type < sortedRels[j].Type
	})

	for _, r := range sortedRels {
		fmt.Fprintf(&b, "  %q -> %q [label=%q];\n", r.FromID, r.ToID, string(r.Type))
	}

	b.WriteString("}\n")
	return b.String()
}

// mermaidBrackets maps entity types to Mermaid node bracket styles.
// Each entry is [open, close] pair.
var mermaidBrackets = map[model.EntityType][2]string{
	model.EntityTypeRequirement: {"[\"", "\"]"},
	model.EntityTypeDecision:    {"{\"", "\"}"},
	model.EntityTypePhase:       {"([\"", "\"])"},
	model.EntityTypeInterface:   {"{{\"", "\"}}"},
	model.EntityTypeState:       {"[[\"", "\"]]"},
	model.EntityTypeTest:        {"([\"", "\"])"},
	model.EntityTypeCrosscut:    {"[/\"", "\"/]"},
	model.EntityTypeQuestion:    {">\"", "\"]"},
	model.EntityTypeAssumption:  {"(\"", "\")"},
	model.EntityTypeCriterion:   {"((\"", "\"))"},
	model.EntityTypeRisk:        {"[/\"", "\"\\]"},
}

// ExportMermaid renders entities and relations as a Mermaid flowchart LR diagram.
// Output is deterministic: nodes sorted by ID, edges by (from_id, to_id, type).
func ExportMermaid(entities []model.Entity, relations []model.Relation) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	sorted := make([]model.Entity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, e := range sorted {
		brackets := mermaidBrackets[e.Type]
		if brackets == [2]string{} {
			brackets = [2]string{"[\"", "\"]"}
		}
		title := mermaidEscape(e.Title)
		fmt.Fprintf(&b, "  %s%s%s: %s%s\n", e.ID, brackets[0], e.ID, title, brackets[1])
	}

	sortedRels := make([]model.Relation, len(relations))
	copy(sortedRels, relations)
	sort.Slice(sortedRels, func(i, j int) bool {
		if sortedRels[i].FromID != sortedRels[j].FromID {
			return sortedRels[i].FromID < sortedRels[j].FromID
		}
		if sortedRels[i].ToID != sortedRels[j].ToID {
			return sortedRels[i].ToID < sortedRels[j].ToID
		}
		return sortedRels[i].Type < sortedRels[j].Type
	})

	for _, r := range sortedRels {
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", r.FromID, mermaidEscape(string(r.Type)), r.ToID)
	}

	return b.String()
}

// mermaidEscape escapes special characters for Mermaid diagram text.
func mermaidEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "[", "&#91;")
	s = strings.ReplaceAll(s, "]", "&#93;")
	s = strings.ReplaceAll(s, "|", "&#124;")
	return s
}

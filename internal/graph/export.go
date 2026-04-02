package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// ExportOptions controls optional filtering and styling for export functions.
type ExportOptions struct {
	// Layer restricts output to entities/relations in this layer. nil = all layers.
	Layer *model.Layer
}

// filterByLayer returns filtered copies of entities and relations matching the given layer.
// If layer is nil, the original slices are returned unchanged.
func filterByLayer(entities []model.Entity, relations []model.Relation, layer *model.Layer) ([]model.Entity, []model.Relation) {
	if layer == nil {
		return entities, relations
	}
	l := *layer
	idSet := make(map[string]bool)
	var fe []model.Entity
	for _, e := range entities {
		if model.LayerForEntityType(e.Type) == l {
			fe = append(fe, e)
			idSet[e.ID] = true
		}
	}
	var fr []model.Relation
	for _, r := range relations {
		rl := model.LayerForRelationType(r.Type)
		if rl == l {
			fr = append(fr, r)
		} else if l != model.LayerMapping && rl == model.LayerMapping && idSet[r.FromID] && idSet[r.ToID] {
			// Include mapping relations when both endpoints are in the filtered set.
			fr = append(fr, r)
		}
	}
	return fe, fr
}

// dotFillColor maps layers to Graphviz fill colors.
var dotFillColor = map[model.Layer]string{
	model.LayerArch:    "lightblue",
	model.LayerExec:    "lightyellow",
	model.LayerMapping: "lavender",
}

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

func ExportDOT(entities []model.Entity, relations []model.Relation, opts *ExportOptions) string {
	if opts != nil {
		entities, relations = filterByLayer(entities, relations, opts.Layer)
	}

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
		fill := dotFillColor[model.LayerForEntityType(e.Type)]
		if fill == "" {
			fill = "white"
		}
		fmt.Fprintf(&b, "  %q [label=%q shape=%s style=filled fillcolor=%s];\n", e.ID, e.ID+"\n"+e.Title, shape, fill)
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
		if model.LayerForRelationType(r.Type) == model.LayerMapping {
			fmt.Fprintf(&b, "  %q -> %q [label=%q style=dashed];\n", r.FromID, r.ToID, string(r.Type))
		} else {
			fmt.Fprintf(&b, "  %q -> %q [label=%q];\n", r.FromID, r.ToID, string(r.Type))
		}
	}

	b.WriteString("}\n")
	return b.String()
}

var mermaidLayerClass = map[model.Layer]string{
	model.LayerArch: "arch",
	model.LayerExec: "exec",
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

func ExportMermaid(entities []model.Entity, relations []model.Relation, opts *ExportOptions) string {
	if opts != nil {
		entities, relations = filterByLayer(entities, relations, opts.Layer)
	}

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
		layer := model.LayerForEntityType(e.Type)
		cls := mermaidLayerClass[layer]
		if cls != "" {
			fmt.Fprintf(&b, "  %s%s%s: %s%s:::%s\n", e.ID, brackets[0], e.ID, title, brackets[1], cls)
		} else {
			fmt.Fprintf(&b, "  %s%s%s: %s%s\n", e.ID, brackets[0], e.ID, title, brackets[1])
		}
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
		if model.LayerForRelationType(r.Type) == model.LayerMapping {
			fmt.Fprintf(&b, "  %s -.->|%s| %s\n", r.FromID, mermaidEscape(string(r.Type)), r.ToID)
		} else {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", r.FromID, mermaidEscape(string(r.Type)), r.ToID)
		}
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

func ExportJSON(entities []model.Entity, relations []model.Relation, opts *ExportOptions) jsoncontract.ExportJSONResult {
	if opts != nil {
		entities, relations = filterByLayer(entities, relations, opts.Layer)
	}

	sorted := make([]model.Entity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	jsonEntities := make([]jsoncontract.ExportJSONEntity, len(sorted))
	for i, e := range sorted {
		var meta map[string]interface{}
		if len(e.Metadata) > 0 {
			_ = json.Unmarshal(e.Metadata, &meta)
		}
		if meta == nil {
			meta = map[string]interface{}{}
		}
		jsonEntities[i] = jsoncontract.ExportJSONEntity{
			ID:       e.ID,
			Type:     string(e.Type),
			Title:    e.Title,
			Status:   string(e.Status),
			Layer:    string(model.LayerForEntityType(e.Type)),
			Metadata: meta,
		}
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

	jsonRelations := make([]jsoncontract.ExportJSONRelation, len(sortedRels))
	for i, r := range sortedRels {
		jsonRelations[i] = jsoncontract.ExportJSONRelation{
			FromID: r.FromID,
			ToID:   r.ToID,
			Type:   string(r.Type),
			Layer:  string(model.LayerForRelationType(r.Type)),
			Weight: r.Weight,
		}
	}

	return jsoncontract.ExportJSONResult{
		Entities:  jsonEntities,
		Relations: jsonRelations,
	}
}

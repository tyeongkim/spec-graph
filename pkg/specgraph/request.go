package specgraph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// Request field parsing lives here rather than in each transport. The CLI, RPC,
// and MCP surfaces all reach the engine through these request types, so
// validating the fields here is what makes the three agree: previously each
// surface parsed layers and severities itself, and RPC skipped the checks the
// other two applied.

// parseLayer converts a request's layer field into an optional layer filter. An
// empty string means all layers.
func parseLayer(layer string) (*model.Layer, error) {
	if layer == "" || layer == "all" {
		return nil, nil
	}
	l := model.Layer(layer)
	if !model.IsValidLayer(l) {
		return nil, newError(CodeInvalidInput,
			fmt.Sprintf("invalid layer %q; must be %s", layer, joinLayers()), nil)
	}
	return &l, nil
}

// parseLayerString validates a layer the same way as parseLayer but returns the
// canonical string form, where empty means all layers.
func parseLayerString(layer string) (string, error) {
	l, err := parseLayer(layer)
	if err != nil {
		return "", err
	}
	if l == nil {
		return "", nil
	}
	return string(*l), nil
}

// orList renders names as a human-readable alternation: "a, b, or c".
func orList(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
}

func joinLayers() string {
	names := make([]string, 0, len(model.ValidLayers)+1)
	for _, l := range model.ValidLayers {
		names = append(names, string(l))
	}
	names = append(names, "all")
	return orList(names)
}

// unresolvedTypes are the entity types that can be in an unresolved state, and
// so the only valid filters for an unresolved query.
var unresolvedTypes = []model.EntityType{
	model.EntityTypeQuestion,
	model.EntityTypeAssumption,
	model.EntityTypeRisk,
}

// parseUnresolvedType validates an unresolved-query type filter. An empty string
// means every unresolved type.
func parseUnresolvedType(entityType string) (*model.EntityType, error) {
	if entityType == "" {
		return nil, nil
	}
	et := model.EntityType(entityType)
	if slices.Contains(unresolvedTypes, et) {
		return &et, nil
	}

	names := make([]string, len(unresolvedTypes))
	for i, t := range unresolvedTypes {
		names[i] = string(t)
	}
	return nil, newError(CodeInvalidInput,
		fmt.Sprintf("invalid type %q; must be %s", entityType, orList(names)), nil)
}

// parseSeverity validates an impact severity floor. An empty string means no floor.
func parseSeverity(severity string) (*graph.Severity, error) {
	if severity == "" {
		return nil, nil
	}
	s := graph.Severity(severity)
	switch s {
	case graph.SeverityHigh, graph.SeverityMedium, graph.SeverityLow:
		return &s, nil
	}
	return nil, newError(CodeInvalidInput,
		fmt.Sprintf("invalid min_severity %q; must be %s, %s, or %s",
			severity, graph.SeverityHigh, graph.SeverityMedium, graph.SeverityLow), nil)
}

// parseDimension validates an impact dimension. An empty string means all dimensions.
func parseDimension(dimension string) (*graph.Dimension, error) {
	if dimension == "" {
		return nil, nil
	}
	d := graph.Dimension(dimension)
	switch d {
	case graph.DimensionStructural, graph.DimensionBehavioral, graph.DimensionPlanning:
		return &d, nil
	}
	return nil, newError(CodeInvalidInput,
		fmt.Sprintf("invalid dimension %q; must be %s, %s, or %s",
			dimension, graph.DimensionStructural, graph.DimensionBehavioral, graph.DimensionPlanning), nil)
}

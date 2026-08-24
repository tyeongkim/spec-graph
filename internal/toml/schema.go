// Package spectoml provides parsing and validation of spec-graph schema.toml files.
// The schema is a serializable view of the entity and relation vocabulary that
// internal/model owns; it is derived from the model rather than restated, so the
// two cannot drift.
package spectoml

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/model"
)

// Schema represents the full spec-graph schema definition.
type Schema struct {
	Version       int                           `toml:"version"`
	EntityTypes   map[string]EntityTypeConfig   `toml:"entity_types"`
	RelationTypes map[string]RelationTypeConfig `toml:"relation_types"`
}

// EntityTypeConfig defines a single entity type in the schema.
type EntityTypeConfig struct {
	Prefix        string   `toml:"prefix"`
	Layer         string   `toml:"layer"`
	AllowedStatus []string `toml:"allowed_status"`
}

// RelationTypeConfig defines a single relation type in the schema.
type RelationTypeConfig struct {
	Layer   string               `toml:"layer"`
	From    []string             `toml:"from"`
	To      []string             `toml:"to"`
	Pairs   []RelationPairConfig `toml:"pairs"`
	Special string               `toml:"special"`
}

type RelationPairConfig struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

// DefaultSchema returns the schema describing the built-in model vocabulary.
// Every field is read from internal/model, which owns the prefix, layer, status,
// and edge rules.
func DefaultSchema() *Schema {
	entityTypes := make(map[string]EntityTypeConfig, len(model.TypePrefixMap))
	for et, prefix := range model.TypePrefixMap {
		statuses := model.AllowedStatuses(et)
		allowed := make([]string, len(statuses))
		for i, s := range statuses {
			allowed[i] = string(s)
		}
		entityTypes[string(et)] = EntityTypeConfig{
			Prefix:        prefix,
			Layer:         string(model.LayerForEntityType(et)),
			AllowedStatus: allowed,
		}
	}

	relationTypes := make(map[string]RelationTypeConfig, len(model.ValidRelationTypes))
	for _, rt := range model.ValidRelationTypes {
		relationTypes[string(rt)] = relationConfig(rt)
	}

	return &Schema{
		Version:       1,
		EntityTypes:   entityTypes,
		RelationTypes: relationTypes,
	}
}

// relationConfig derives one relation's rule by asking the model which endpoint
// pairs it permits, so the schema view cannot disagree with the edge matrix.
func relationConfig(rt model.RelationType) RelationTypeConfig {
	layer := model.LayerForRelationType(rt)
	cfg := RelationTypeConfig{Layer: string(layer)}

	if special, ok := specialRelations[rt]; ok {
		cfg.Special = special
		return cfg
	}

	var pairs []RelationPairConfig
	fromSet := make(map[model.EntityType]bool)
	toSet := make(map[model.EntityType]bool)
	for _, from := range sortedEntityTypes() {
		for _, to := range sortedEntityTypes() {
			if !model.IsEdgeAllowed(rt, from, to, &layer) {
				continue
			}
			pairs = append(pairs, RelationPairConfig{From: string(from), To: string(to)})
			fromSet[from] = true
			toSet[to] = true
		}
	}

	// When the permitted pairs are exactly the cross product of the endpoints,
	// from/to expresses the rule more compactly and matches how schema.toml is
	// written. Otherwise the pairs must be listed to stay exact.
	if len(pairs) == len(fromSet)*len(toSet) {
		cfg.From = entityTypeNames(fromSet)
		cfg.To = entityTypeNames(toSet)
		return cfg
	}

	cfg.Pairs = pairs
	return cfg
}

// specialRelations are the relations whose rule is not a set of endpoint types.
// They mirror the special cases in model.IsEdgeAllowed.
var specialRelations = map[model.RelationType]string{
	model.RelationSupersedes:    "same_type",
	model.RelationConflictsWith: "any_to_any",
	model.RelationReferences:    "any_to_any",
}

func sortedEntityTypes() []model.EntityType {
	types := slices.Clone(model.ValidEntityTypes)
	slices.Sort(types)
	return types
}

func entityTypeNames(set map[model.EntityType]bool) []string {
	names := make([]string, 0, len(set))
	for et := range set {
		names = append(names, string(et))
	}
	slices.Sort(names)
	return names
}

// ValidateEntity checks that the entity ID matches the expected prefix for the
// given entity type and that the status is allowed.
func (s *Schema) ValidateEntity(id string, entityType, status string) error {
	if _, ok := s.EntityTypes[entityType]; !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}
	return model.ValidateEntity(id, model.EntityType(entityType), model.EntityStatus(status))
}

// ValidateRelation checks whether a relation of the given type is permitted
// between the specified entity types.
func (s *Schema) ValidateRelation(fromType, toType, relationType string) error {
	cfg, ok := s.RelationTypes[relationType]
	if !ok {
		return fmt.Errorf("unknown relation type %q", relationType)
	}

	switch cfg.Special {
	case "same_type":
		if fromType != toType {
			return fmt.Errorf("relation %q requires same entity type on both sides; got %q → %q",
				relationType, fromType, toType)
		}
		return nil
	case "any_to_any":
		return nil
	}
	if len(cfg.Pairs) > 0 {
		if slices.ContainsFunc(cfg.Pairs, func(pair RelationPairConfig) bool {
			return pair.From == fromType && pair.To == toType
		}) {
			return nil
		}
		return fmt.Errorf("relation %q does not allow exact pair %q → %q", relationType, fromType, toType)
	}

	if !slices.Contains(cfg.From, fromType) {
		return fmt.Errorf("entity type %q is not a valid source for relation %q; allowed: %s",
			fromType, relationType, strings.Join(cfg.From, ", "))
	}

	if !slices.Contains(cfg.To, toType) {
		return fmt.Errorf("entity type %q is not a valid target for relation %q; allowed: %s",
			toType, relationType, strings.Join(cfg.To, ", "))
	}

	return nil
}

// IsRelationAllowed is a convenience method that returns true if the relation is valid.
func (s *Schema) IsRelationAllowed(fromType, toType, relationType string) bool {
	return s.ValidateRelation(fromType, toType, relationType) == nil
}

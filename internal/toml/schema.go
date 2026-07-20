// Package spectoml provides parsing and validation of spec-graph schema.toml files.
// The schema defines allowed entity types, relation types, edge matrix rules,
// and status values — making the graph model self-describing and validatable.
package spectoml

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
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

var entityIDPattern = regexp.MustCompile(`^([A-Z]+)-([0-9]+)(?:-([0-9a-z]{3}))?$`)

// LoadSchema reads and parses a schema.toml file from the given path.
func LoadSchema(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}

	return ParseSchema(data)
}

// ParseSchema parses schema.toml content from raw bytes.
func ParseSchema(data []byte) (*Schema, error) {
	var s Schema
	if err := toml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	if err := s.validate(); err != nil {
		return nil, err
	}

	return &s, nil
}

// DefaultSchema returns the built-in default schema matching the current edge_matrix.go logic.
func DefaultSchema() *Schema {
	return &Schema{
		Version: 1,
		EntityTypes: map[string]EntityTypeConfig{
			"requirement": {Prefix: "REQ", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"decision":    {Prefix: "DEC", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"phase":       {Prefix: "PHS", Layer: "exec", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"plan":        {Prefix: "PLN", Layer: "exec", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"interface":   {Prefix: "API", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"state":       {Prefix: "STT", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"test":        {Prefix: "TST", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"crosscut":    {Prefix: "XCT", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"question":    {Prefix: "QST", Layer: "arch", AllowedStatus: []string{"draft", "active", "resolved", "deleted"}},
			"assumption":  {Prefix: "ASM", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"criterion":   {Prefix: "ACT", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"risk":        {Prefix: "RSK", Layer: "arch", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"change":      {Prefix: "CHG", Layer: "exec", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
			"task":        {Prefix: "TSK", Layer: "exec", AllowedStatus: []string{"draft", "active", "deprecated", "resolved", "deleted"}},
		},
		RelationTypes: map[string]RelationTypeConfig{
			"implements":      {Layer: "arch", From: []string{"interface"}, To: []string{"requirement", "criterion"}},
			"verifies":        {Layer: "arch", From: []string{"test"}, To: []string{"requirement", "criterion", "decision", "interface", "state"}},
			"depends_on":      {Layer: "arch", From: []string{"requirement", "decision", "interface", "test", "state"}, To: []string{"requirement", "decision", "interface", "state", "crosscut", "assumption"}},
			"constrained_by":  {Layer: "arch", From: []string{"requirement", "decision", "interface", "state"}, To: []string{"crosscut", "decision", "assumption"}},
			"triggers":        {Layer: "arch", From: []string{"interface", "decision"}, To: []string{"state"}},
			"answers":         {Layer: "arch", From: []string{"decision"}, To: []string{"question"}},
			"assumes":         {Layer: "arch", From: []string{"requirement", "decision", "interface"}, To: []string{"assumption"}},
			"has_criterion":   {Layer: "arch", From: []string{"requirement"}, To: []string{"criterion"}},
			"mitigates":       {Layer: "arch", From: []string{"decision", "test", "crosscut"}, To: []string{"risk"}},
			"supersedes":      {Layer: "arch", Special: "same_type"},
			"conflicts_with":  {Layer: "arch", Special: "any_to_any"},
			"references":      {Layer: "arch", Special: "any_to_any"},
			"belongs_to":      {Layer: "exec", Pairs: []RelationPairConfig{{From: "phase", To: "plan"}, {From: "task", To: "phase"}}},
			"precedes":        {Layer: "exec", From: []string{"phase"}, To: []string{"phase"}},
			"blocks":          {Layer: "exec", From: []string{"phase"}, To: []string{"phase"}},
			"task_depends_on": {Layer: "exec", From: []string{"task"}, To: []string{"task"}},
			"covers":          {Layer: "mapping", From: []string{"phase", "change", "task"}, To: []string{"requirement", "decision", "interface", "test", "question", "risk", "criterion", "assumption"}},
			"delivers":        {Layer: "mapping", From: []string{"phase"}, To: []string{"requirement", "interface", "state", "test", "decision", "criterion"}},
		},
	}
}

// ValidateEntity checks that the entity ID matches the expected prefix for the
// given entity type and that the status is allowed.
func (s *Schema) ValidateEntity(id string, entityType, status string) error {
	cfg, ok := s.EntityTypes[entityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}

	matches := entityIDPattern.FindStringSubmatch(id)
	if matches == nil {
		return fmt.Errorf("entity ID %q does not match format PREFIX-NNN", id)
	}

	prefix := matches[1]
	if prefix != cfg.Prefix {
		return fmt.Errorf("entity ID %q has prefix %q; expected %q for type %q", id, prefix, cfg.Prefix, entityType)
	}

	if !slices.Contains(cfg.AllowedStatus, status) {
		return fmt.Errorf("status %q is not allowed for entity type %q; allowed: %s",
			status, entityType, strings.Join(cfg.AllowedStatus, ", "))
	}

	return nil
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

// validate performs internal consistency checks on the parsed schema.
func (s *Schema) validate() error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported schema version %d; expected 1", s.Version)
	}

	if len(s.EntityTypes) == 0 {
		return fmt.Errorf("schema must define at least one entity type")
	}

	if len(s.RelationTypes) == 0 {
		return fmt.Errorf("schema must define at least one relation type")
	}

	validLayers := []string{"arch", "exec", "mapping"}

	prefixes := make(map[string]string, len(s.EntityTypes))
	for name, cfg := range s.EntityTypes {
		if cfg.Prefix == "" {
			return fmt.Errorf("entity type %q: prefix must not be empty", name)
		}
		if existing, dup := prefixes[cfg.Prefix]; dup {
			return fmt.Errorf("entity type %q: prefix %q already used by %q", name, cfg.Prefix, existing)
		}
		prefixes[cfg.Prefix] = name

		if !slices.Contains(validLayers, cfg.Layer) {
			return fmt.Errorf("entity type %q: invalid layer %q", name, cfg.Layer)
		}
		if len(cfg.AllowedStatus) == 0 {
			return fmt.Errorf("entity type %q: allowed_status must not be empty", name)
		}
	}

	for name, cfg := range s.RelationTypes {
		if !slices.Contains(validLayers, cfg.Layer) {
			return fmt.Errorf("relation type %q: invalid layer %q", name, cfg.Layer)
		}

		if cfg.Special != "" {
			validSpecials := []string{"same_type", "any_to_any"}
			if !slices.Contains(validSpecials, cfg.Special) {
				return fmt.Errorf("relation type %q: invalid special value %q", name, cfg.Special)
			}
			continue
		}

		if len(cfg.Pairs) > 0 {
			for _, pair := range cfg.Pairs {
				if _, ok := s.EntityTypes[pair.From]; !ok {
					return fmt.Errorf("relation type %q: pair references unknown source entity type %q", name, pair.From)
				}
				if _, ok := s.EntityTypes[pair.To]; !ok {
					return fmt.Errorf("relation type %q: pair references unknown target entity type %q", name, pair.To)
				}
			}
			continue
		}

		if len(cfg.From) == 0 {
			return fmt.Errorf("relation type %q: from must not be empty", name)
		}
		if len(cfg.To) == 0 {
			return fmt.Errorf("relation type %q: to must not be empty", name)
		}

		for _, et := range cfg.From {
			if _, ok := s.EntityTypes[et]; !ok {
				return fmt.Errorf("relation type %q: from references unknown entity type %q", name, et)
			}
		}
		for _, et := range cfg.To {
			if _, ok := s.EntityTypes[et]; !ok {
				return fmt.Errorf("relation type %q: to references unknown entity type %q", name, et)
			}
		}
	}

	return nil
}

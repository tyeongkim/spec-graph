package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type EntityType string

const (
	EntityTypeRequirement EntityType = "requirement"
	EntityTypeDecision    EntityType = "decision"
	EntityTypePhase       EntityType = "phase"
	EntityTypeInterface   EntityType = "interface"
	EntityTypeState       EntityType = "state"
	EntityTypeTest        EntityType = "test"
	EntityTypeCrosscut    EntityType = "crosscut"
	EntityTypeQuestion    EntityType = "question"
	EntityTypeAssumption  EntityType = "assumption"
	EntityTypeCriterion   EntityType = "criterion"
	EntityTypeRisk        EntityType = "risk"
)

var TypePrefixMap = map[EntityType]string{
	EntityTypeRequirement: "REQ",
	EntityTypeDecision:    "DEC",
	EntityTypePhase:       "PHS",
	EntityTypeInterface:   "API",
	EntityTypeState:       "STT",
	EntityTypeTest:        "TST",
	EntityTypeCrosscut:    "XCT",
	EntityTypeQuestion:    "QST",
	EntityTypeAssumption:  "ASM",
	EntityTypeCriterion:   "ACT",
	EntityTypeRisk:        "RSK",
}

type EntityStatus string

const (
	EntityStatusDraft      EntityStatus = "draft"
	EntityStatusActive     EntityStatus = "active"
	EntityStatusDeprecated EntityStatus = "deprecated"
	EntityStatusResolved   EntityStatus = "resolved"
	EntityStatusDeleted    EntityStatus = "deleted"
)

type Entity struct {
	ID          string          `json:"id"`
	Type        EntityType      `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Status      EntityStatus    `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

var entityIDPattern = regexp.MustCompile(`^([A-Z]+)-(\d+)$`)

func ValidateEntityID(id string, entityType EntityType) error {
	if id == "" {
		return fmt.Errorf("entity ID must not be empty")
	}

	matches := entityIDPattern.FindStringSubmatch(id)
	if matches == nil {
		return fmt.Errorf("entity ID %q does not match format PREFIX-NNN", id)
	}

	prefix := matches[1]
	expectedPrefix, ok := TypePrefixMap[entityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}

	if !strings.EqualFold(prefix, expectedPrefix) || prefix != expectedPrefix {
		return fmt.Errorf("entity ID %q has prefix %q; expected %q for type %q", id, prefix, expectedPrefix, entityType)
	}

	return nil
}

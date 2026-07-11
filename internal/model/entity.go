package model

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
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
	EntityTypePlan        EntityType = "plan"
	EntityTypeChange      EntityType = "change"
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
	EntityTypePlan:        "PLN",
	EntityTypeChange:      "CHG",
}

// PrefixTypeMap is the reverse of TypePrefixMap: prefix string → EntityType.
var PrefixTypeMap = func() map[string]EntityType {
	m := make(map[string]EntityType, len(TypePrefixMap))
	for et, prefix := range TypePrefixMap {
		m[prefix] = et
	}
	return m
}()

// ValidEntityTypes contains all recognized entity types.
var ValidEntityTypes = func() []EntityType {
	types := make([]EntityType, 0, len(TypePrefixMap))
	for et := range TypePrefixMap {
		types = append(types, et)
	}
	return types
}()

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
	Layer       Layer           `json:"layer"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Status      EntityStatus    `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

var entityIDPattern = regexp.MustCompile(`^([A-Z]+)-([0-9]+)(?:-([0-9a-z]{3}))?$`)

// idAlphabet is the Crockford Base32 lowercase alphabet with the ambiguous
// characters i, l, o, and u removed. It is used to build the random suffix of
// decentralized entity IDs.
const idAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// ParseEntityID splits an entity id into its prefix, numeric value, and the
// character width of the numeric segment (width detects zero-padding: "REQ-001"
// has width 3). It accepts both legacy IDs of the form PREFIX-NNN and new-form
// IDs of the form PREFIX-<unixSeconds>-<rand3>. For new-form IDs, num and width
// are derived from the unix-seconds segment; those values are only meaningful
// for legacy sequential numbering and are no longer used to generate IDs. ok is
// false when id matches neither form.
func ParseEntityID(id string) (prefix string, num int, width int, ok bool) {
	matches := entityIDPattern.FindStringSubmatch(id)
	if matches == nil {
		return "", 0, 0, false
	}
	digits := matches[2]
	n, err := strconv.Atoi(digits)
	if err != nil {
		return "", 0, 0, false
	}
	return matches[1], n, len(digits), true
}

// GenerateEntityID returns a new decentralized, sortable entity ID of the form
// PREFIX-<unixSeconds>-<rand3>. The unix-seconds prefix makes IDs sort
// chronologically as strings; the random suffix avoids collisions across
// concurrent or branch-parallel creation without central coordination.
func GenerateEntityID(et EntityType) (string, error) {
	prefix, ok := TypePrefixMap[et]
	if !ok {
		return "", fmt.Errorf("unknown entity type %q", et)
	}

	suffix, err := randomSuffix(3)
	if err != nil {
		return "", fmt.Errorf("generate random ID suffix: %w", err)
	}

	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().Unix(), suffix), nil
}

// randomSuffix returns a string of n characters drawn uniformly from
// idAlphabet using crypto/rand as the entropy source.
func randomSuffix(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return string(out), nil
}

func ValidateEntityID(id string, entityType EntityType) error {
	if id == "" {
		return fmt.Errorf("entity ID must not be empty")
	}

	prefix, _, _, ok := ParseEntityID(id)
	if !ok {
		return fmt.Errorf("entity ID %q does not match format PREFIX-NNN", id)
	}

	expectedPrefix, ok := TypePrefixMap[entityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}

	if prefix != expectedPrefix {
		return fmt.Errorf("entity ID %q has prefix %q; expected %q for type %q", id, prefix, expectedPrefix, entityType)
	}

	return nil
}

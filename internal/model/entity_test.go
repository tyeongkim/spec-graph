package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEntityTypeConstants(t *testing.T) {
	expected := map[EntityType]string{
		EntityTypeRequirement: "requirement",
		EntityTypeDecision:    "decision",
		EntityTypePhase:       "phase",
		EntityTypeInterface:   "interface",
		EntityTypeState:       "state",
		EntityTypeTest:        "test",
		EntityTypeCrosscut:    "crosscut",
		EntityTypeQuestion:    "question",
		EntityTypeAssumption:  "assumption",
		EntityTypeCriterion:   "criterion",
		EntityTypeRisk:        "risk",
		EntityTypePlan:        "plan",
	}

	if len(expected) != 12 {
		t.Fatalf("expected 12 entity types, got %d", len(expected))
	}

	for et, want := range expected {
		if string(et) != want {
			t.Errorf("EntityType %q != %q", et, want)
		}
	}
}

func TestTypePrefixMap(t *testing.T) {
	expected := map[EntityType]string{
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
	}

	for et, wantPrefix := range expected {
		got, ok := TypePrefixMap[et]
		if !ok {
			t.Errorf("TypePrefixMap missing key %q", et)
			continue
		}
		if got != wantPrefix {
			t.Errorf("TypePrefixMap[%q] = %q; want %q", et, got, wantPrefix)
		}
	}

	if len(TypePrefixMap) != 12 {
		t.Errorf("TypePrefixMap has %d entries; want 12", len(TypePrefixMap))
	}
}

func TestEntityStatusConstants(t *testing.T) {
	expected := map[EntityStatus]string{
		EntityStatusDraft:      "draft",
		EntityStatusActive:     "active",
		EntityStatusDeprecated: "deprecated",
		EntityStatusResolved:   "resolved",
		EntityStatusDeleted:    "deleted",
	}

	if len(expected) != 5 {
		t.Fatalf("expected 5 entity statuses, got %d", len(expected))
	}

	for es, want := range expected {
		if string(es) != want {
			t.Errorf("EntityStatus %q != %q", es, want)
		}
	}
}

func TestPrefixTypeMap(t *testing.T) {
	if len(PrefixTypeMap) != len(TypePrefixMap) {
		t.Fatalf("PrefixTypeMap has %d entries; TypePrefixMap has %d", len(PrefixTypeMap), len(TypePrefixMap))
	}

	for et, prefix := range TypePrefixMap {
		got, ok := PrefixTypeMap[prefix]
		if !ok {
			t.Errorf("PrefixTypeMap missing key %q", prefix)
			continue
		}
		if got != et {
			t.Errorf("PrefixTypeMap[%q] = %q; want %q", prefix, got, et)
		}
	}
}

func TestValidEntityTypes(t *testing.T) {
	if len(ValidEntityTypes) != 12 {
		t.Fatalf("ValidEntityTypes has %d entries; want 12", len(ValidEntityTypes))
	}

	seen := make(map[EntityType]bool)
	for _, et := range ValidEntityTypes {
		if seen[et] {
			t.Errorf("duplicate entity type %q in ValidEntityTypes", et)
		}
		seen[et] = true
		if _, ok := TypePrefixMap[et]; !ok {
			t.Errorf("ValidEntityTypes contains %q which is not in TypePrefixMap", et)
		}
	}
}

func TestValidateEntityID(t *testing.T) {
	validCases := []struct {
		name       string
		id         string
		entityType EntityType
	}{
		{"requirement", "REQ-001", EntityTypeRequirement},
		{"decision", "DEC-003", EntityTypeDecision},
		{"phase", "PHS-002", EntityTypePhase},
		{"interface", "API-005", EntityTypeInterface},
		{"state", "STT-001", EntityTypeState},
		{"test", "TST-012", EntityTypeTest},
		{"crosscut", "XCT-002", EntityTypeCrosscut},
		{"question", "QST-004", EntityTypeQuestion},
		{"assumption", "ASM-003", EntityTypeAssumption},
		{"criterion", "ACT-009", EntityTypeCriterion},
		{"risk", "RSK-002", EntityTypeRisk},
		{"plan", "PLN-001", EntityTypePlan},
	}

	for _, tc := range validCases {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := ValidateEntityID(tc.id, tc.entityType); err != nil {
				t.Errorf("ValidateEntityID(%q, %q) unexpected error: %v", tc.id, tc.entityType, err)
			}
		})
	}

	invalidCases := []struct {
		name       string
		id         string
		entityType EntityType
	}{
		{"empty string", "", EntityTypeRequirement},
		{"missing number", "REQ-", EntityTypeRequirement},
		{"lowercase prefix", "req-001", EntityTypeRequirement},
		{"wrong prefix for type", "DEC-001", EntityTypeRequirement},
		{"no dash", "REQ001", EntityTypeRequirement},
		{"letters after dash", "REQ-ABC", EntityTypeRequirement},
		{"extra dash segments", "REQ-001-002", EntityTypeRequirement},
		{"only prefix", "REQ", EntityTypeRequirement},
		{"space in id", "REQ -001", EntityTypeRequirement},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			if err := ValidateEntityID(tc.id, tc.entityType); err == nil {
				t.Errorf("ValidateEntityID(%q, %q) expected error, got nil", tc.id, tc.entityType)
			}
		})
	}
}

func TestEntityStruct(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	meta := json.RawMessage(`{"priority":"must"}`)

	e := Entity{
		ID:          "REQ-001",
		Type:        EntityTypeRequirement,
		Layer:       LayerArch,
		Title:       "Test Requirement",
		Description: "A test requirement",
		Status:      EntityStatusDraft,
		Metadata:    meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if e.ID != "REQ-001" {
		t.Errorf("ID = %q; want %q", e.ID, "REQ-001")
	}
	if e.Type != EntityTypeRequirement {
		t.Errorf("Type = %q; want %q", e.Type, EntityTypeRequirement)
	}
	if e.Status != EntityStatusDraft {
		t.Errorf("Status = %q; want %q", e.Status, EntityStatusDraft)
	}

	// Verify JSON marshaling preserves metadata
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Entity
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != e.ID {
		t.Errorf("decoded ID = %q; want %q", decoded.ID, e.ID)
	}
	if string(decoded.Metadata) != string(e.Metadata) {
		t.Errorf("decoded Metadata = %s; want %s", decoded.Metadata, e.Metadata)
	}
}

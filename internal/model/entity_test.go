package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
		EntityTypeChange:      "change",
		EntityTypeTask:        "task",
	}

	if len(expected) != 14 {
		t.Fatalf("expected 14 entity types, got %d", len(expected))
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
		EntityTypeChange:      "CHG",
		EntityTypeTask:        "TSK",
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

	if len(TypePrefixMap) != 14 {
		t.Errorf("TypePrefixMap has %d entries; want 14", len(TypePrefixMap))
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
	if len(ValidEntityTypes) != 14 {
		t.Fatalf("ValidEntityTypes has %d entries; want 14", len(ValidEntityTypes))
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
		{"task", "TSK-001", EntityTypeTask},
		{"new-form requirement", "REQ-1752239482-k3f", EntityTypeRequirement},
		{"new-form decision", "DEC-1752239482-0zz", EntityTypeDecision},
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
		{"suffix too short", "REQ-001-ab", EntityTypeRequirement},
		{"suffix too long", "REQ-001-abcd", EntityTypeRequirement},
		{"suffix uppercase", "REQ-001-AB1", EntityTypeRequirement},
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

func TestParseEntityID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantOK    bool
		wantPref  string
		wantNum   int
		wantWidth int
	}{
		{"unpadded", "REQ-1", true, "REQ", 1, 1},
		{"padded width 3", "REQ-001", true, "REQ", 1, 3},
		{"padded high", "REQ-042", true, "REQ", 42, 3},
		{"wide number", "REQ-1000", true, "REQ", 1000, 4},
		{"other prefix", "DEC-7", true, "DEC", 7, 1},
		{"new-form", "REQ-1752239482-k3f", true, "REQ", 1752239482, 10},
		{"new-form other prefix", "DEC-1752239482-0zz", true, "DEC", 1752239482, 10},
		{"empty", "", false, "", 0, 0},
		{"no number", "REQ-", false, "", 0, 0},
		{"no dash", "REQ001", false, "", 0, 0},
		{"letters after dash", "REQ-AB", false, "", 0, 0},
		{"suffix too long", "REQ-1-abcd", false, "", 0, 0},
		{"suffix uppercase", "REQ-1-AB1", false, "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, num, width, ok := ParseEntityID(tt.id)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v; want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if prefix != tt.wantPref || num != tt.wantNum || width != tt.wantWidth {
				t.Errorf("ParseEntityID(%q) = (%q, %d, %d); want (%q, %d, %d)",
					tt.id, prefix, num, width, tt.wantPref, tt.wantNum, tt.wantWidth)
			}
		})
	}
}

func TestGenerateEntityID(t *testing.T) {
	t.Run("valid for every type", func(t *testing.T) {
		for et, prefix := range TypePrefixMap {
			id, err := GenerateEntityID(et)
			if err != nil {
				t.Fatalf("GenerateEntityID(%q): %v", et, err)
			}
			if err := ValidateEntityID(id, et); err != nil {
				t.Errorf("generated ID %q for %q failed validation: %v", id, et, err)
			}
			gotPrefix, _, _, ok := ParseEntityID(id)
			if !ok {
				t.Fatalf("generated ID %q for %q did not parse", id, et)
			}
			if gotPrefix != prefix {
				t.Errorf("generated ID %q has prefix %q; want %q", id, gotPrefix, prefix)
			}
		}
	})

	t.Run("unknown type errors", func(t *testing.T) {
		if _, err := GenerateEntityID(EntityType("bogus")); err == nil {
			t.Error("expected error for unknown entity type, got nil")
		}
	})

	t.Run("format shape", func(t *testing.T) {
		id, err := GenerateEntityID(EntityTypeRequirement)
		if err != nil {
			t.Fatalf("GenerateEntityID: %v", err)
		}
		parts := strings.Split(id, "-")
		if len(parts) != 3 {
			t.Fatalf("ID %q has %d dash-segments; want 3", id, len(parts))
		}
		if parts[0] != "REQ" {
			t.Errorf("prefix = %q; want %q", parts[0], "REQ")
		}
		secs, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			t.Errorf("middle segment %q is not an integer: %v", parts[1], err)
		}
		if secs <= 0 {
			t.Errorf("unix-seconds segment = %d; want positive", secs)
		}
		if len(parts[2]) != 3 {
			t.Errorf("random suffix %q has length %d; want 3", parts[2], len(parts[2]))
		}
		for _, r := range parts[2] {
			if !strings.ContainsRune(idAlphabet, r) {
				t.Errorf("random suffix %q contains char %q outside alphabet", parts[2], r)
			}
		}
	})

	t.Run("collisions are rare within a second", func(t *testing.T) {
		const n = 1000
		seen := make(map[string]bool, n)
		dupes := 0
		for i := 0; i < n; i++ {
			id, err := GenerateEntityID(EntityTypeRequirement)
			if err != nil {
				t.Fatalf("GenerateEntityID: %v", err)
			}
			if seen[id] {
				dupes++
			}
			seen[id] = true
		}
		if dupes > n/20 {
			t.Errorf("got %d duplicate IDs out of %d; suffix entropy unexpectedly low", dupes, n)
		}
	})

	t.Run("chronological string sort", func(t *testing.T) {
		earlier := fmt.Sprintf("REQ-%d-aaa", int64(1000000000))
		later := fmt.Sprintf("REQ-%d-aaa", int64(2000000000))
		if !(earlier < later) {
			t.Errorf("expected %q < %q lexicographically", earlier, later)
		}
	})
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

package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewOutputPreservesCandidates(t *testing.T) {
	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Layer: "arch", Title: "Auth", Confidence: 0.9, Source: "spec.md#L1"},
		},
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.8, Source: "spec.md#L2"},
		},
	}

	data, err := json.Marshal(ReviewCandidates(input))
	if err != nil {
		t.Fatalf("marshal review output: %v", err)
	}
	const want = `{"entities":[{"id":"REQ-001","type":"requirement","layer":"arch","title":"Auth","confidence":0.9,"source":"spec.md#L1"}],"relations":[{"from":"REQ-001","to":"DEC-001","type":"depends_on","confidence":0.8,"source":"spec.md#L2"}]}`
	if string(data) != want {
		t.Errorf("review JSON = %s; want %s", data, want)
	}
}

func TestLoadCandidatesFromFile(t *testing.T) {
	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Test", Confidence: 0.9, Source: "test.md#L1"},
		},
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.8, Source: "test.md#L5"},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "candidates.json")
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := LoadCandidatesFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadCandidatesFromFile: %v", err)
	}

	if len(got.Entities) != 1 || got.Entities[0].ID != "REQ-001" {
		t.Errorf("entities mismatch: got %+v", got.Entities)
	}
	if len(got.Relations) != 1 || got.Relations[0].From != "REQ-001" {
		t.Errorf("relations mismatch: got %+v", got.Relations)
	}
}

func TestLoadCandidatesFromFile_NotFound(t *testing.T) {
	_, err := LoadCandidatesFromFile("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadCandidatesFromFile_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmpFile, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, err := LoadCandidatesFromFile(tmpFile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

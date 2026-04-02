package bootstrap

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/db"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/store"
)

func setupImportTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.OpenMemoryDB()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func newTestStores(t *testing.T, d *sql.DB) (*store.EntityStore, *store.RelationStore) {
	t.Helper()
	cs := store.NewChangesetStore(d)
	hs := store.NewHistoryStore(d)
	return store.NewEntityStore(d, cs, hs), store.NewRelationStore(d, cs, hs)
}

func TestReviewCandidates(t *testing.T) {
	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Auth", Confidence: 0.9},
		},
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.8},
		},
	}

	got := ReviewCandidates(input)

	if len(got.Entities) != 1 || got.Entities[0].ID != "REQ-001" {
		t.Errorf("entities mismatch: got %+v", got.Entities)
	}
	if len(got.Relations) != 1 || got.Relations[0].From != "REQ-001" {
		t.Errorf("relations mismatch: got %+v", got.Relations)
	}
}

func TestApplyCandidates_CreatesEntities(t *testing.T) {
	d := setupImportTestDB(t)
	es, rs := newTestStores(t, d)

	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Layer: "arch", Title: "Auth required", Confidence: 0.9},
			{ID: "DEC-001", Type: "decision", Layer: "arch", Title: "Use JWT", Confidence: 0.8},
		},
	}

	result := ApplyCandidates(input, es, rs)

	if len(result.Created) != 2 {
		t.Fatalf("expected 2 created, got %d: %v", len(result.Created), result.Created)
	}

	e, err := es.Get("REQ-001")
	if err != nil {
		t.Fatalf("get REQ-001: %v", err)
	}
	if e.Title != "Auth required" {
		t.Errorf("title = %q; want %q", e.Title, "Auth required")
	}
	if e.Type != model.EntityTypeRequirement {
		t.Errorf("type = %q; want %q", e.Type, model.EntityTypeRequirement)
	}
	if e.Layer != model.LayerArch {
		t.Errorf("layer = %q; want %q", e.Layer, model.LayerArch)
	}
}

func TestApplyCandidates_SkipsExisting(t *testing.T) {
	d := setupImportTestDB(t)
	es, rs := newTestStores(t, d)

	_, err := es.Create(model.Entity{
		ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Pre-existing",
	}, "seed", "", "test")
	if err != nil {
		t.Fatalf("seed entity: %v", err)
	}

	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Duplicate", Confidence: 0.9},
		},
	}

	result := ApplyCandidates(input, es, rs)

	if len(result.Created) != 0 {
		t.Errorf("expected 0 created, got %d", len(result.Created))
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if result.Skipped[0].Reason != "already exists" {
		t.Errorf("reason = %q; want %q", result.Skipped[0].Reason, "already exists")
	}
}

func TestApplyCandidates_SkipsLowConfidence(t *testing.T) {
	d := setupImportTestDB(t)
	es, rs := newTestStores(t, d)

	input := ScanResult{
		Entities: []EntityCandidate{
			{ID: "REQ-001", Type: "requirement", Title: "Low conf", Confidence: 0.3},
		},
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.4},
		},
	}

	result := ApplyCandidates(input, es, rs)

	if len(result.Created) != 0 {
		t.Errorf("expected 0 created, got %d", len(result.Created))
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %d", len(result.Skipped))
	}
	for _, s := range result.Skipped {
		if s.Reason != "low confidence" {
			t.Errorf("reason = %q; want %q", s.Reason, "low confidence")
		}
	}
}

func TestApplyCandidates_SkipsInvalidEdge(t *testing.T) {
	d := setupImportTestDB(t)
	es, rs := newTestStores(t, d)

	// REQ implements REQ is not allowed (implements: interface → requirement|criterion)
	_, err := es.Create(model.Entity{
		ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "R1",
	}, "seed", "", "test")
	if err != nil {
		t.Fatalf("seed REQ-001: %v", err)
	}
	_, err = es.Create(model.Entity{
		ID: "REQ-002", Type: model.EntityTypeRequirement, Title: "R2",
	}, "seed", "", "test")
	if err != nil {
		t.Fatalf("seed REQ-002: %v", err)
	}

	input := ScanResult{
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "REQ-002", Type: "implements", Confidence: 0.8},
		},
	}

	result := ApplyCandidates(input, es, rs)

	if len(result.Created) != 0 {
		t.Errorf("expected 0 created, got %d", len(result.Created))
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if result.Skipped[0].Reason != "invalid edge" {
		t.Errorf("reason = %q; want %q", result.Skipped[0].Reason, "invalid edge")
	}
}

func TestApplyCandidates_CreatesRelations(t *testing.T) {
	d := setupImportTestDB(t)
	es, rs := newTestStores(t, d)

	_, err := es.Create(model.Entity{
		ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "R1",
	}, "seed", "", "test")
	if err != nil {
		t.Fatalf("seed REQ-001: %v", err)
	}
	_, err = es.Create(model.Entity{
		ID: "DEC-001", Type: model.EntityTypeDecision, Title: "D1",
	}, "seed", "", "test")
	if err != nil {
		t.Fatalf("seed DEC-001: %v", err)
	}

	input := ScanResult{
		Relations: []RelationCandidate{
			{From: "REQ-001", To: "DEC-001", Type: "depends_on", Confidence: 0.8},
		},
	}

	result := ApplyCandidates(input, es, rs)

	if len(result.Created) != 1 {
		t.Fatalf("expected 1 created, got %d", len(result.Created))
	}
	if result.Created[0] != "REQ-001:DEC-001:depends_on" {
		t.Errorf("created key = %q; want %q", result.Created[0], "REQ-001:DEC-001:depends_on")
	}

	rels, err := rs.GetByEntity("REQ-001")
	if err != nil {
		t.Fatalf("get relations: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation in DB, got %d", len(rels))
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

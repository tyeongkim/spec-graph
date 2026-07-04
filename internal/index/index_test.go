package index

import (
	"os"
	"path/filepath"
	"testing"
)

func testEntities() []EntityRecord {
	return []EntityRecord{
		{ID: "REQ-001", Type: "requirement", Layer: "spec", Status: "active", Title: "User authentication", FilePath: "spec/req-001.toml"},
		{ID: "REQ-002", Type: "requirement", Layer: "spec", Status: "draft", Title: "Password reset flow", FilePath: "spec/req-002.toml"},
		{ID: "DEC-001", Type: "decision", Layer: "design", Status: "active", Title: "Adopt JWT tokens", FilePath: "design/dec-001.toml"},
		{ID: "INT-001", Type: "interface", Layer: "design", Status: "deprecated", Title: "Auth API endpoint", FilePath: "design/int-001.toml"},
	}
}

func testRelations() []RelationRecord {
	return []RelationRecord{
		{FromID: "DEC-001", ToID: "REQ-001", Type: "implements", Layer: "design", Weight: 1.0},
		{FromID: "INT-001", ToID: "DEC-001", Type: "realizes", Layer: "design", Weight: 0.8},
		{FromID: "REQ-002", ToID: "REQ-001", Type: "depends_on", Layer: "spec", Weight: 1.0},
	}
}

func openTestIndex(t *testing.T) *Index {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	idx, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.db")

	idx, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
}

func TestRebuildAndQuery(t *testing.T) {
	idx := openTestIndex(t)

	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	e, err := idx.GetEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if e == nil {
		t.Fatal("GetEntity returned nil for REQ-001")
	}
	if e.Title != "User authentication" {
		t.Errorf("Title = %q, want %q", e.Title, "User authentication")
	}
	if e.Type != "requirement" {
		t.Errorf("Type = %q, want %q", e.Type, "requirement")
	}

	missing, err := idx.GetEntity("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetEntity nonexistent: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent entity")
	}
}

func TestRebuildAtomic(t *testing.T) {
	idx := openTestIndex(t)

	oldEntities := []EntityRecord{
		{ID: "OLD-001", Type: "requirement", Layer: "spec", Status: "active", Title: "Old entity", FilePath: "old.toml"},
	}
	if err := idx.Rebuild(oldEntities, nil); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}

	e, err := idx.GetEntity("OLD-001")
	if err != nil {
		t.Fatalf("GetEntity OLD-001: %v", err)
	}
	if e == nil {
		t.Fatal("OLD-001 should exist after first rebuild")
	}

	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}

	e, err = idx.GetEntity("OLD-001")
	if err != nil {
		t.Fatalf("GetEntity OLD-001 after second rebuild: %v", err)
	}
	if e != nil {
		t.Error("OLD-001 should be gone after second rebuild")
	}

	e, err = idx.GetEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetEntity REQ-001: %v", err)
	}
	if e == nil {
		t.Fatal("REQ-001 should exist after second rebuild")
	}
}

func TestListEntitiesFilters(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	tests := []struct {
		name    string
		filters EntityFilters
		wantIDs []string
	}{
		{
			name:    "by type",
			filters: EntityFilters{Type: "requirement"},
			wantIDs: []string{"REQ-001", "REQ-002"},
		},
		{
			name:    "by status",
			filters: EntityFilters{Status: "active"},
			wantIDs: []string{"DEC-001", "REQ-001"},
		},
		{
			name:    "by layer",
			filters: EntityFilters{Layer: "design"},
			wantIDs: []string{"DEC-001", "INT-001"},
		},
		{
			name:    "combined type+status",
			filters: EntityFilters{Type: "requirement", Status: "draft"},
			wantIDs: []string{"REQ-002"},
		},
		{
			name:    "no filters returns all",
			filters: EntityFilters{},
			wantIDs: []string{"DEC-001", "INT-001", "REQ-001", "REQ-002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idx.ListEntities(tt.filters)
			if err != nil {
				t.Fatalf("ListEntities: %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d entities, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestListRelationsFilters(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	tests := []struct {
		name    string
		filters RelationFilters
		want    int
	}{
		{name: "by from_id", filters: RelationFilters{FromID: "DEC-001"}, want: 1},
		{name: "by to_id", filters: RelationFilters{ToID: "REQ-001"}, want: 2},
		{name: "by type", filters: RelationFilters{Type: "implements"}, want: 1},
		{name: "by layer", filters: RelationFilters{Layer: "design"}, want: 2},
		{name: "no filters", filters: RelationFilters{}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idx.ListRelations(tt.filters)
			if err != nil {
				t.Fatalf("ListRelations: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d relations, want %d", len(got), tt.want)
			}
		})
	}
}

func TestGetRelationsByEntity(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	got, err := idx.GetRelationsByEntity("DEC-001")
	if err != nil {
		t.Fatalf("GetRelationsByEntity: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d relations, want 2 (one outgoing, one incoming)", len(got))
	}

	got, err = idx.GetRelationsByEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetRelationsByEntity REQ-001: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d relations for REQ-001, want 2", len(got))
	}

	got, err = idx.GetRelationsByEntity("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetRelationsByEntity nonexistent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d relations for nonexistent, want 0", len(got))
	}
}

func TestSearchEntities(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	got, err := idx.SearchEntities("authentication")
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].ID != "REQ-001" {
		t.Errorf("got ID %q, want REQ-001", got[0].ID)
	}

	got, err = idx.SearchEntities("JWT")
	if err != nil {
		t.Fatalf("SearchEntities JWT: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d results for JWT, want 1", len(got))
	}
	if got[0].ID != "DEC-001" {
		t.Errorf("got ID %q, want DEC-001", got[0].ID)
	}

	got, err = idx.SearchEntities("nonexistent_term_xyz")
	if err != nil {
		t.Fatalf("SearchEntities nonexistent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d results for nonexistent term, want 0", len(got))
	}
}

func TestMetaGetSet(t *testing.T) {
	idx := openTestIndex(t)

	val, err := idx.GetMeta("fingerprint")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for missing key, got %q", val)
	}

	if err := idx.SetMeta("fingerprint", "abc123"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	val, err = idx.GetMeta("fingerprint")
	if err != nil {
		t.Fatalf("GetMeta after set: %v", err)
	}
	if val != "abc123" {
		t.Errorf("got %q, want %q", val, "abc123")
	}

	if err := idx.SetMeta("fingerprint", "def456"); err != nil {
		t.Fatalf("SetMeta upsert: %v", err)
	}

	val, err = idx.GetMeta("fingerprint")
	if err != nil {
		t.Fatalf("GetMeta after upsert: %v", err)
	}
	if val != "def456" {
		t.Errorf("got %q, want %q", val, "def456")
	}
}

func TestRelationWeightDefault(t *testing.T) {
	idx := openTestIndex(t)

	relations := []RelationRecord{
		{FromID: "A", ToID: "B", Type: "test", Layer: "spec", Weight: 0},
	}
	entities := []EntityRecord{
		{ID: "A", Type: "requirement", Layer: "spec", Status: "active", Title: "A", FilePath: "a.toml"},
		{ID: "B", Type: "requirement", Layer: "spec", Status: "active", Title: "B", FilePath: "b.toml"},
	}

	if err := idx.Rebuild(entities, relations); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	got, err := idx.ListRelations(RelationFilters{})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d relations, want 1", len(got))
	}
	if got[0].Weight != 1.0 {
		t.Errorf("Weight = %f, want 1.0 (default)", got[0].Weight)
	}
}

func TestRefreshIfReplacedDetectsSwap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	defer first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer second.Close()

	if err := second.Rebuild(testEntities(), testRelations()); err != nil {
		t.Fatalf("second Rebuild (simulating another process): %v", err)
	}

	replaced, err := first.RefreshIfReplaced()
	if err != nil {
		t.Fatalf("RefreshIfReplaced: %v", err)
	}
	if !replaced {
		t.Fatal("expected RefreshIfReplaced to report a swap after another handle rebuilt the file")
	}

	got, err := first.ListEntities(EntityFilters{})
	if err != nil {
		t.Fatalf("ListEntities after refresh: %v", err)
	}
	if len(got) != len(testEntities()) {
		t.Errorf("got %d entities after self-heal, want %d", len(got), len(testEntities()))
	}
}

func TestRefreshIfReplacedNoopWhenUnchanged(t *testing.T) {
	idx := openTestIndex(t)

	replaced, err := idx.RefreshIfReplaced()
	if err != nil {
		t.Fatalf("RefreshIfReplaced: %v", err)
	}
	if replaced {
		t.Error("expected no reopen when the file was not replaced")
	}
}

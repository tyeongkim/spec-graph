package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

func setupTest(t *testing.T) (*Syncer, *spectoml.Store, *index.Index, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".spec-graph")
	store := spectoml.NewStore(root)
	if err := store.Init(); err != nil {
		t.Fatalf("store.Init: %v", err)
	}

	dbPath := filepath.Join(root, "graph.db")
	idx, err := index.Open(dbPath)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	syncer := NewSyncer(store, idx, root)
	return syncer, store, idx, root
}

func writeEntity(t *testing.T, store *spectoml.Store, id string, entityType model.EntityType, title string, relations []spectoml.RelationEntry) {
	t.Helper()
	ef := &spectoml.EntityFile{
		Schema:    1,
		ID:        id,
		Type:      entityType,
		Title:     title,
		Status:    model.EntityStatusActive,
		Relations: relations,
	}
	if err := store.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity %s: %v", id, err)
	}
}

func TestEnsureFresh_NoFiles_RebuildsOnce(t *testing.T) {
	syncer, _, _, _ := setupTest(t)

	rebuilt, err := syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if !rebuilt {
		t.Error("expected rebuild on first call with no files")
	}

	rebuilt, err = syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh second call: %v", err)
	}
	if rebuilt {
		t.Error("expected no rebuild on second call (nothing changed)")
	}
}

func TestEnsureFresh_AfterAddingFile(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	if _, err := syncer.EnsureFresh(); err != nil {
		t.Fatalf("initial EnsureFresh: %v", err)
	}

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)

	rebuilt, err := syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh after add: %v", err)
	}
	if !rebuilt {
		t.Error("expected rebuild after adding a file")
	}

	rebuilt, err = syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh after stable: %v", err)
	}
	if rebuilt {
		t.Error("expected no rebuild when nothing changed")
	}
}

func TestEnsureFresh_AfterModifyingFile(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)

	if _, err := syncer.EnsureFresh(); err != nil {
		t.Fatalf("initial EnsureFresh: %v", err)
	}

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth v2", nil)

	rebuilt, err := syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh after modify: %v", err)
	}
	if !rebuilt {
		t.Error("expected rebuild after modifying a file")
	}
}

func TestEnsureFresh_AfterDeletingFile(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)
	writeEntity(t, store, "DEC-001", model.EntityTypeDecision, "JWT", nil)

	if _, err := syncer.EnsureFresh(); err != nil {
		t.Fatalf("initial EnsureFresh: %v", err)
	}

	if err := store.DeleteEntity("REQ-001", model.EntityTypeRequirement); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	rebuilt, err := syncer.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh after delete: %v", err)
	}
	if !rebuilt {
		t.Error("expected rebuild after deleting a file")
	}
}

func TestForceRebuild_PopulatesIndex(t *testing.T) {
	syncer, store, idx, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", []spectoml.RelationEntry{
		{To: "DEC-001", Type: model.RelationDependsOn},
	})
	writeEntity(t, store, "DEC-001", model.EntityTypeDecision, "JWT", nil)

	if err := syncer.ForceRebuild(); err != nil {
		t.Fatalf("ForceRebuild: %v", err)
	}

	e, err := idx.GetEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetEntity REQ-001: %v", err)
	}
	if e == nil {
		t.Fatal("REQ-001 not found in index after rebuild")
	}
	if e.Title != "Auth" {
		t.Errorf("Title = %q, want %q", e.Title, "Auth")
	}
	if e.Type != "requirement" {
		t.Errorf("Type = %q, want %q", e.Type, "requirement")
	}
	if e.Layer != "arch" {
		t.Errorf("Layer = %q, want %q", e.Layer, "arch")
	}

	e, err = idx.GetEntity("DEC-001")
	if err != nil {
		t.Fatalf("GetEntity DEC-001: %v", err)
	}
	if e == nil {
		t.Fatal("DEC-001 not found in index after rebuild")
	}

	rels, err := idx.ListRelations(index.RelationFilters{FromID: "REQ-001"})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("got %d relations, want 1", len(rels))
	}
	if rels[0].ToID != "DEC-001" {
		t.Errorf("relation ToID = %q, want %q", rels[0].ToID, "DEC-001")
	}
	if rels[0].Type != "depends_on" {
		t.Errorf("relation Type = %q, want %q", rels[0].Type, "depends_on")
	}
}

func TestFingerprint_Deterministic(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)
	writeEntity(t, store, "DEC-001", model.EntityTypeDecision, "JWT", nil)

	fp1, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint 1: %v", err)
	}

	fp2, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint 2: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("fingerprints differ: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_ChangesOnContentChange(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)

	fp1, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint before: %v", err)
	}

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth v2", nil)

	fp2, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint after: %v", err)
	}

	if fp1 == fp2 {
		t.Error("fingerprint should change when file content changes")
	}
}

func TestFingerprint_NotAffectedByMtime(t *testing.T) {
	syncer, store, _, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)

	fp1, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint 1: %v", err)
	}

	path := store.EntityPath("REQ-001", model.EntityTypeRequirement)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile (same content, new mtime): %v", err)
	}

	fp2, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint 2: %v", err)
	}

	if fp1 != fp2 {
		t.Error("fingerprint should not change when only mtime changes")
	}
}

func TestForceRebuild_PartialParseFailure(t *testing.T) {
	syncer, store, idx, root := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Good entity", nil)

	badDir := filepath.Join(root, "entities", "decision")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("create bad dir: %v", err)
	}
	badPath := filepath.Join(badDir, "DEC-BAD.toml")
	if err := os.WriteFile(badPath, []byte("this is not valid toml [[["), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	err := syncer.ForceRebuild()
	if err == nil {
		t.Fatal("expected error from ForceRebuild with bad file")
	}
	if !IsRebuildError(err) {
		t.Fatalf("expected RebuildError, got %T: %v", err, err)
	}

	e, err := idx.GetEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if e == nil {
		t.Error("good entity should still be in index despite bad file")
	}
}

func TestFingerprint_EmptyState(t *testing.T) {
	syncer, _, _, _ := setupTest(t)

	fp, err := syncer.ComputeFingerprint()
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	if fp == "" {
		t.Error("fingerprint should not be empty even with no files")
	}
	if len(fp) != 64 {
		t.Errorf("fingerprint length = %d, want 64 (sha256 hex)", len(fp))
	}
}

func TestForceRebuild_ClearsOldData(t *testing.T) {
	syncer, store, idx, _ := setupTest(t)

	writeEntity(t, store, "REQ-001", model.EntityTypeRequirement, "Auth", nil)
	if err := syncer.ForceRebuild(); err != nil {
		t.Fatalf("first ForceRebuild: %v", err)
	}

	if err := store.DeleteEntity("REQ-001", model.EntityTypeRequirement); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	writeEntity(t, store, "DEC-001", model.EntityTypeDecision, "JWT", nil)

	if err := syncer.ForceRebuild(); err != nil {
		t.Fatalf("second ForceRebuild: %v", err)
	}

	e, err := idx.GetEntity("REQ-001")
	if err != nil {
		t.Fatalf("GetEntity REQ-001: %v", err)
	}
	if e != nil {
		t.Error("REQ-001 should be gone after rebuild without it")
	}

	e, err = idx.GetEntity("DEC-001")
	if err != nil {
		t.Fatalf("GetEntity DEC-001: %v", err)
	}
	if e == nil {
		t.Error("DEC-001 should exist after rebuild")
	}
}

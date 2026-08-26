package specgraph

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestTxnCommitFailureRestoresModifiedFiles(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not constrain root")
	}

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "requirement", ID: "REQ-001", Title: "First"},
		{Type: "requirement", ID: "REQ-002", Title: "Second"},
		{Type: "requirement", ID: "REQ-003", Title: "Third"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}
	path := eng.store.EntityPath("REQ-001", model.EntityTypeRequirement)
	canonical, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read entity file %q: %v", path, err)
	}
	if err := os.WriteFile(path, append([]byte("# noncanonical TOML\n"), canonical...), 0o644); err != nil {
		t.Fatalf("make entity file %q noncanonical: %v", path, err)
	}
	before := entityFileSnapshot(t, eng.Root())

	dir := filepath.Join(eng.Root(), "entities", "decision")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create decision dir: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make decision dir unwritable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err = transact(eng, func(tx *txn) (struct{}, error) {
		entity, err := tx.read("REQ-001", model.EntityTypeRequirement)
		if err != nil {
			return struct{}{}, err
		}
		entity.Title = "Changed"
		if err := tx.write(entity); err != nil {
			return struct{}{}, err
		}
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-004",
			Title: "Fourth",
		}); err != nil {
			return struct{}{}, err
		}
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "decision",
			ID:    "DEC-001",
			Title: "Failing decision",
		}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err == nil {
		t.Fatal("transact error = nil, want commit failure")
	}
	if strings.Contains(err.Error(), "could not restore") {
		t.Errorf("transact error = %v, want original commit failure", err)
	}

	after := entityFileSnapshot(t, eng.Root())
	if len(after) != len(before) {
		t.Errorf("entity file count = %d, want %d", len(after), len(before))
	}
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("entity file %q is missing after rollback", path)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("entity file %q changed after rollback", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("unexpected entity file %q after rollback", path)
		}
	}
}

func TestTxnCommitFailureRollsBackStagedDeletion(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not constrain root")
	}

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "requirement", ID: "REQ-001", Title: "First"},
		{Type: "requirement", ID: "REQ-002", Title: "Second"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}
	before := entityFileSnapshot(t, eng.Root())

	dir := filepath.Join(eng.Root(), "entities", "decision")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create decision dir: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make decision dir unwritable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if err := tx.deleteEntity("REQ-001"); err != nil {
			return struct{}{}, err
		}
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "decision",
			ID:    "DEC-001",
			Title: "Failing decision",
		}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err == nil {
		t.Fatal("transact error = nil, want commit failure")
	}

	after := entityFileSnapshot(t, eng.Root())
	path := eng.store.EntityPath("REQ-001", model.EntityTypeRequirement)
	want, ok := before[path]
	if !ok {
		t.Fatalf("entity file %q is missing from snapshot", path)
	}
	got, ok := after[path]
	if !ok {
		t.Errorf("deleted entity file %q is missing after rollback", path)
	} else if !bytes.Equal(got, want) {
		t.Errorf("deleted entity file %q changed after rollback", path)
	}
}

func TestTxnCommitFailurePropagatesCause(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not constrain root")
	}

	eng := openTestEngine(t)
	dir := filepath.Join(eng.Root(), "entities", "decision")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create decision dir: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make decision dir unwritable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "First",
		}); err != nil {
			return struct{}{}, err
		}
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "decision",
			ID:    "DEC-001",
			Title: "Failing decision",
		}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err == nil {
		t.Fatal("transact error = nil, want commit failure")
	}
	if !strings.Contains(err.Error(), "DEC-001") {
		t.Errorf("transact error = %v, want failing entity ID", err)
	}
	if strings.Contains(err.Error(), "could not restore") {
		t.Errorf("transact error = %v, want original commit failure", err)
	}
	if IsConflict(err) {
		t.Errorf("IsConflict(%v) = true, want false", err)
	}
	if IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = true, want false", err)
	}
}

func TestReviseEntityCommitFailureLeavesNoTrace(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not constrain root")
	}

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "requirement", ID: "REQ-001", Title: "Requirement"},
		{Type: "interface", ID: "API-001", Title: "Interface"},
		{Type: "criterion", ID: "ACT-001", Title: "Criterion"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}
	for _, req := range []AddRelationRequest{
		{From: "API-001", To: "REQ-001", Type: "implements"},
		{From: "REQ-001", To: "ACT-001", Type: "has_criterion"},
	} {
		if _, err := eng.AddRelation(ctx, req); err != nil {
			t.Fatalf("AddRelation %q: %v", req.Type, err)
		}
	}
	before := entityFileSnapshot(t, eng.Root())

	dir := filepath.Join(eng.Root(), "entities", "interface")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create interface dir: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make interface dir unwritable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := eng.ReviseEntity(ctx, ReviseEntityRequest{
		ID:     "REQ-001",
		Reason: "Revise the requirement",
	})
	if err == nil {
		t.Fatal("ReviseEntity error = nil, want commit failure")
	}

	after := entityFileSnapshot(t, eng.Root())
	if len(after) != len(before) {
		t.Errorf("entity file count = %d, want %d", len(after), len(before))
	}
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("entity file %q is missing after rollback", path)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("entity file %q changed after rollback", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("unexpected entity file %q after rollback", path)
		}
	}
}

// TestReviseEntityRebuildsIndexOnce pins the cost reduction that motivated the
// transaction: revise used to rebuild the index once per moved relation. Each
// rebuild renames a fresh database over graph.db, so a replaced file identifies
// a rebuild.
func TestReviseEntityRebuildsIndexOnce(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "requirement", ID: "REQ-001", Title: "Requirement"},
		{Type: "interface", ID: "API-001", Title: "Interface"},
		{Type: "test", ID: "TST-001", Title: "Test"},
		{Type: "criterion", ID: "ACT-001", Title: "Criterion"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}
	for _, req := range []AddRelationRequest{
		{From: "API-001", To: "REQ-001", Type: "implements"},
		{From: "TST-001", To: "REQ-001", Type: "verifies"},
		{From: "REQ-001", To: "ACT-001", Type: "has_criterion"},
	} {
		if _, err := eng.AddRelation(ctx, req); err != nil {
			t.Fatalf("AddRelation %q: %v", req.Type, err)
		}
	}

	dbPath := filepath.Join(eng.Root(), "graph.db")
	beforeStaging := statFile(t, dbPath)

	tx := &txn{eng: eng, staged: make(map[string]*stagedFile)}
	if _, err := tx.reviseEntity(ReviseEntityRequest{
		ID:     "REQ-001",
		Reason: "Revise the requirement",
	}); err != nil {
		t.Fatalf("reviseEntity: %v", err)
	}

	afterStaging := statFile(t, dbPath)
	if !os.SameFile(beforeStaging, afterStaging) {
		t.Error("index was rebuilt while relations were still staged")
	}
	if len(tx.order) < 4 {
		t.Fatalf("staged entity count = %d, want at least 4", len(tx.order))
	}

	if err := tx.commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	afterCommit := statFile(t, dbPath)
	if os.SameFile(afterStaging, afterCommit) {
		t.Error("commit did not rebuild the index")
	}
}

func statFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info
}

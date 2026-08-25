package specgraph

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

func newInitializedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatalf("create entities dir: %v", err)
	}
	return root
}

func openTestEngine(t *testing.T) *Engine {
	t.Helper()
	root := newInitializedRoot(t)
	eng, err := Open(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func TestTxnStagedEntityVisibleBeforeCommit(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	path := eng.store.EntityPath("REQ-001", model.EntityTypeRequirement)

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "Staged requirement",
		}); err != nil {
			return struct{}{}, err
		}

		entity, err := (&stagedEntityFetcher{tx: tx}).Get("REQ-001")
		if err != nil {
			return struct{}{}, err
		}
		if entity.ID != "REQ-001" {
			t.Errorf("entityFetcher.Get ID = %q, want %q", entity.ID, "REQ-001")
		}
		if !tx.exists("REQ-001", model.EntityTypeRequirement) {
			t.Error("txn.exists returned false for a staged entity")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("staged entity file exists before commit: %v", err)
		}

		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}
}

func TestTxnDuplicateEntityIDConflicts(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "First requirement",
		}); err != nil {
			return struct{}{}, err
		}

		_, createErr := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "Second requirement",
		})
		if !IsConflict(createErr) {
			t.Errorf("second create error = %v, want conflict", createErr)
		}

		entity, err := tx.read("REQ-001", model.EntityTypeRequirement)
		if err != nil {
			return struct{}{}, err
		}
		if entity.Title != "First requirement" {
			t.Errorf("staged entity title = %q, want %q", entity.Title, "First requirement")
		}

		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}

	files := entityFileSnapshot(t, eng.Root())
	if len(files) != 1 {
		t.Errorf("entity file count = %d, want 1", len(files))
	}
	entity, err := eng.store.ReadEntity("REQ-001", model.EntityTypeRequirement)
	if err != nil {
		t.Fatalf("read committed entity: %v", err)
	}
	if entity.Title != "First requirement" {
		t.Errorf("committed entity title = %q, want %q", entity.Title, "First requirement")
	}
}

func TestTxnDeletedRelationDoesNotReappearInTargetLookup(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "interface", ID: "API-001", Title: "Source"},
		{Type: "requirement", ID: "REQ-001", Title: "Target"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}
	if _, err := eng.AddRelation(ctx, AddRelationRequest{
		From: "API-001",
		To:   "REQ-001",
		Type: "implements",
	}); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if err := tx.deleteRelation(DeleteRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}); err != nil {
			return struct{}{}, err
		}

		relations, err := (&stagedRelationFetcher{tx: tx}).GetByEntity("REQ-001")
		if err != nil {
			return struct{}{}, err
		}
		for _, relation := range relations {
			if relation.FromID == "API-001" && relation.ToID == "REQ-001" && relation.Type == model.RelationImplements {
				t.Error("deleted relation appeared in the target-side lookup")
			}
		}

		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}
}

func TestTxnStagedRelationVisibleFromTarget(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{Type: "interface", ID: "API-001", Title: "Source"},
		{Type: "requirement", ID: "REQ-001", Title: "Target"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatalf("CreateEntity %q: %v", req.ID, err)
		}
	}

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if _, err := tx.addRelation(AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}); err != nil {
			return struct{}{}, err
		}

		relations, err := (&stagedRelationFetcher{tx: tx}).GetByEntity("REQ-001")
		if err != nil {
			return struct{}{}, err
		}
		if len(relations) != 1 {
			t.Errorf("target-side relation count = %d, want 1", len(relations))
			return struct{}{}, nil
		}
		relation := relations[0]
		if relation.FromID != "API-001" {
			t.Errorf("relation FromID = %q, want %q", relation.FromID, "API-001")
		}
		if relation.ToID != "REQ-001" {
			t.Errorf("relation ToID = %q, want %q", relation.ToID, "REQ-001")
		}
		if relation.Type != model.RelationImplements {
			t.Errorf("relation Type = %q, want %q", relation.Type, model.RelationImplements)
		}

		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}
}

func TestTxnStagedEndpointsDoNotDuplicateRelation(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		for _, req := range []CreateEntityRequest{
			{Type: "interface", ID: "API-001", Title: "Source"},
			{Type: "requirement", ID: "REQ-001", Title: "Target"},
		} {
			if _, err := tx.createEntity(req); err != nil {
				return struct{}{}, err
			}
		}
		if _, err := tx.addRelation(AddRelationRequest{
			From: "API-001",
			To:   "REQ-001",
			Type: "implements",
		}); err != nil {
			return struct{}{}, err
		}

		for _, id := range []string{"API-001", "REQ-001"} {
			relations, err := (&stagedRelationFetcher{tx: tx}).GetByEntity(id)
			if err != nil {
				return struct{}{}, err
			}
			if len(relations) != 1 {
				t.Errorf("relations for %q = %d, want 1", id, len(relations))
				continue
			}
			relation := relations[0]
			if relation.FromID != "API-001" || relation.ToID != "REQ-001" || relation.Type != model.RelationImplements {
				t.Errorf("relation for %q = %s->%s[%s], want API-001->REQ-001[implements]", id, relation.FromID, relation.ToID, relation.Type)
			}
		}

		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}
}

func TestTxnCreateThenDeleteIsNoOp(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	path := eng.store.EntityPath("REQ-001", model.EntityTypeRequirement)

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if _, err := tx.createEntity(CreateEntityRequest{
			Type:  "requirement",
			ID:    "REQ-001",
			Title: "Temporary requirement",
		}); err != nil {
			return struct{}{}, err
		}
		if err := tx.deleteEntity("REQ-001"); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("entity file exists after create-then-delete transaction: %v", err)
	}
}

func TestTxnFnErrorLeavesEntityFilesUnchanged(t *testing.T) {
	t.Parallel()

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
	rollbackCause := errors.New("abort transaction")

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		entity, err := tx.read("REQ-001", model.EntityTypeRequirement)
		if err != nil {
			return struct{}{}, err
		}
		entity.Title = "Changed"
		if err := tx.write(entity); err != nil {
			return struct{}{}, err
		}
		for _, req := range []CreateEntityRequest{
			{Type: "requirement", ID: "REQ-003", Title: "Third"},
			{Type: "decision", ID: "DEC-001", Title: "Decision"},
		} {
			if _, err := tx.createEntity(req); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, rollbackCause
	})
	if !errors.Is(err, rollbackCause) {
		t.Fatalf("transact error = %v, want %v", err, rollbackCause)
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

func TestTxnReadReturnsIsolatedClone(t *testing.T) {
	t.Parallel()

	eng := openTestEngine(t)
	rollbackCause := errors.New("abort transaction")

	_, err := transact(eng, func(tx *txn) (struct{}, error) {
		if err := tx.write(&spectoml.EntityFile{
			Schema: 1,
			ID:     "REQ-001",
			Type:   model.EntityTypeRequirement,
			Title:  "Requirement",
			Status: model.EntityStatusDraft,
			Metadata: map[string]any{
				"owner": "original",
			},
			Relations: []spectoml.RelationEntry{
				{
					To:       "REQ-002",
					Type:     model.RelationDependsOn,
					Metadata: map[string]any{"reason": "original"},
				},
			},
		}); err != nil {
			return struct{}{}, err
		}

		first, err := tx.read("REQ-001", model.EntityTypeRequirement)
		if err != nil {
			return struct{}{}, err
		}
		first.Metadata["owner"] = "changed"
		first.Relations[0].Metadata["reason"] = "changed"

		second, err := tx.read("REQ-001", model.EntityTypeRequirement)
		if err != nil {
			return struct{}{}, err
		}
		if second.Metadata["owner"] != "original" {
			t.Errorf("second read metadata owner = %v, want %q", second.Metadata["owner"], "original")
		}
		if second.Relations[0].Metadata["reason"] != "original" {
			t.Errorf("second read relation metadata reason = %v, want %q", second.Relations[0].Metadata["reason"], "original")
		}

		return struct{}{}, rollbackCause
	})
	if !errors.Is(err, rollbackCause) {
		t.Fatalf("transact error = %v, want %v", err, rollbackCause)
	}
}

func entityFileSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()

	files := make(map[string][]byte)
	err := filepath.WalkDir(filepath.Join(root, "entities"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot entity files: %v", err)
	}
	return files
}

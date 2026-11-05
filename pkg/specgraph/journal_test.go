package specgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestInterruptedCommitRestoresSourceAndIndex(t *testing.T) {
	for _, reopen := range []bool{true, false} {
		name := "next_operation"
		if reopen {
			name = "open"
		}
		t.Run(name, func(t *testing.T) {
			eng := openTestEngine(t)
			ctx := context.Background()
			for _, id := range []string{"REQ-001", "REQ-002"} {
				if _, err := eng.CreateEntity(ctx, CreateEntityRequest{Type: "requirement", ID: id, Title: "Original"}); err != nil {
					t.Fatal(err)
				}
			}
			originals := make(map[string][]byte)
			var entries []map[string]any
			for _, id := range []string{"REQ-001", "REQ-002"} {
				path := eng.store.EntityPath(id, model.EntityTypeRequirement)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				originals[path] = data
				entries = append(entries, map[string]any{"id": id, "entity_type": "requirement", "pre_image": data, "absent": false})
			}
			entries = append(entries, map[string]any{"id": "REQ-003", "entity_type": "requirement", "absent": true})
			journal, err := json.Marshal(map[string]any{"version": 1, "entries": entries})
			if err != nil {
				t.Fatal(err)
			}
			journalPath := filepath.Join(eng.Root(), "txn.journal")
			if err := os.WriteFile(journalPath, journal, 0o600); err != nil {
				t.Fatal(err)
			}
			first := eng.store.EntityPath("REQ-001", model.EntityTypeRequirement)
			if err := os.WriteFile(first, bytes.ReplaceAll(originals[first], []byte("Original"), []byte("Interrupted")), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := eng.store.DeleteEntity("REQ-002", model.EntityTypeRequirement); err != nil {
				t.Fatal(err)
			}
			if err := eng.store.WriteEntityBytes("REQ-003", model.EntityTypeRequirement, bytes.ReplaceAll(originals[first], []byte("REQ-001"), []byte("REQ-003"))); err != nil {
				t.Fatal(err)
			}
			if _, err := eng.syncer.EnsureFresh(); err != nil {
				t.Fatal(err)
			}
			if reopen {
				if err := eng.Close(); err != nil {
					t.Fatal(err)
				}
				eng, err = Open(ctx, Options{Root: eng.Root()})
				if err != nil {
					t.Fatal(err)
				}
				defer eng.Close()
			}
			result, err := eng.RawQuery(ctx, "SELECT id, title FROM entities ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Rows) != 2 {
				t.Fatalf("index rows = %v", result.Rows)
			}
			for _, row := range result.Rows {
				if row["title"] != "Original" {
					t.Errorf("index row = %v", row)
				}
			}
			for path, want := range originals {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("restored %s: bytes match = %v, error = %v", path, bytes.Equal(got, want), err)
				}
			}
			if eng.store.EntityExists("REQ-003", model.EntityTypeRequirement) {
				t.Error("interrupted creation survived")
			}
			if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
				t.Errorf("journal survived recovery: %v", err)
			}
		})
	}
}

func TestSuccessfulCommitLeavesNoJournal(t *testing.T) {
	eng := openTestEngine(t)
	if _, err := eng.CreateEntity(context.Background(), CreateEntityRequest{Type: "requirement", Title: "Committed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(eng.Root(), "txn.journal")); !os.IsNotExist(err) {
		t.Fatalf("journal remains after commit: %v", err)
	}
}

func TestUnreadableJournalBlocksOperations(t *testing.T) {
	eng := openTestEngine(t)
	path := filepath.Join(eng.Root(), "txn.journal")
	if err := os.WriteFile(path, []byte("invalid journal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.RawQuery(context.Background(), "SELECT id FROM entities"); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "spec-graph doctor") {
		t.Fatalf("recovery error = %v", err)
	}
	if _, err := Open(context.Background(), Options{Root: eng.Root()}); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "spec-graph doctor") {
		t.Fatalf("open recovery error = %v", err)
	}
}

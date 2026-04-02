package store

import (
	"errors"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func createChangeset(t *testing.T, s *ChangesetStore, reason, actor, source string) model.Changeset {
	t.Helper()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	id, err := s.GetNextID(tx)
	if err != nil {
		tx.Rollback()
		t.Fatalf("GetNextID: %v", err)
	}
	cs, err := s.Create(tx, model.Changeset{
		ID:     id,
		Reason: reason,
		Actor:  actor,
		Source: source,
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("Create changeset: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	return cs
}

func TestChangesetStore_CreateAndGet(t *testing.T) {
	d := setupTestDB(t)
	s := NewChangesetStore(d)

	cs := createChangeset(t, s, "initial import", "alice", "cli")

	if cs.ID != "CHG-1" {
		t.Errorf("ID = %q; want CHG-1", cs.ID)
	}
	if cs.Reason != "initial import" {
		t.Errorf("Reason = %q; want %q", cs.Reason, "initial import")
	}
	if cs.Actor != "alice" {
		t.Errorf("Actor = %q; want %q", cs.Actor, "alice")
	}
	if cs.Source != "cli" {
		t.Errorf("Source = %q; want %q", cs.Source, "cli")
	}
	if cs.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}

	got, err := s.Get("CHG-1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID != cs.ID {
		t.Errorf("Get ID = %q; want %q", got.ID, cs.ID)
	}
	if got.Reason != cs.Reason {
		t.Errorf("Get Reason = %q; want %q", got.Reason, cs.Reason)
	}
	if got.Actor != cs.Actor {
		t.Errorf("Get Actor = %q; want %q", got.Actor, cs.Actor)
	}
	if got.Source != cs.Source {
		t.Errorf("Get Source = %q; want %q", got.Source, cs.Source)
	}
}

func TestChangesetStore_SequentialIDs(t *testing.T) {
	d := setupTestDB(t)
	s := NewChangesetStore(d)

	createChangeset(t, s, "first", "", "")
	createChangeset(t, s, "second", "", "")
	createChangeset(t, s, "third", "", "")

	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	next, err := s.GetNextID(tx)
	if err != nil {
		t.Fatalf("GetNextID: %v", err)
	}
	if next != "CHG-4" {
		t.Errorf("next ID = %q; want CHG-4", next)
	}
}

func TestChangesetStore_List(t *testing.T) {
	d := setupTestDB(t)
	s := NewChangesetStore(d)

	createChangeset(t, s, "first", "", "")
	createChangeset(t, s, "second", "", "")
	createChangeset(t, s, "third", "", "")

	list, err := s.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d; want 3", len(list))
	}

	if list[0].ID != "CHG-3" {
		t.Errorf("list[0].ID = %q; want CHG-3 (DESC order)", list[0].ID)
	}
	if list[2].ID != "CHG-1" {
		t.Errorf("list[2].ID = %q; want CHG-1 (DESC order)", list[2].ID)
	}
}

func TestChangesetStore_GetNotFound(t *testing.T) {
	d := setupTestDB(t)
	s := NewChangesetStore(d)

	_, err := s.Get("CHG-999")
	var nf *model.ErrChangesetNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("expected ErrChangesetNotFound, got %v", err)
	}
}

func TestChangesetStore_NilActorSource(t *testing.T) {
	d := setupTestDB(t)
	s := NewChangesetStore(d)

	cs := createChangeset(t, s, "no actor or source", "", "")

	if cs.Actor != "" {
		t.Errorf("Actor = %q; want empty", cs.Actor)
	}
	if cs.Source != "" {
		t.Errorf("Source = %q; want empty", cs.Source)
	}

	got, err := s.Get(cs.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Actor != "" {
		t.Errorf("Get Actor = %q; want empty", got.Actor)
	}
	if got.Source != "" {
		t.Errorf("Get Source = %q; want empty", got.Source)
	}
}

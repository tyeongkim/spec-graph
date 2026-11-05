package specgraph

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyeongkim/spec-graph/internal/flock"
	"github.com/tyeongkim/spec-graph/internal/index"
)

func TestOpenDoesNotCreateIndexWithoutProjectLock(t *testing.T) {
	root := newTestRoot(t)
	unlock, err := flock.Lock(filepath.Join(root, ".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	eng, err := Open(context.Background(), Options{Root: root, LockTimeout: time.Millisecond})
	if eng != nil {
		eng.Close()
	}
	if !IsConflict(err) {
		t.Fatalf("Open while project locked = %v; want conflict", err)
	}
	if _, err := os.Stat(filepath.Join(root, "graph.db")); !os.IsNotExist(err) {
		t.Fatalf("index created before project lock acquired: %v", err)
	}
}

func TestClosedEngineRejectsReadsAndWritesWithoutRefresh(t *testing.T) {
	eng, err := Open(context.Background(), Options{Root: newTestRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
	eng.lock.onAcquire = func() error {
		t.Fatal("closed engine refreshed the index")
		return nil
	}
	_, _, err = eng.ListEntities(context.Background(), ListEntitiesRequest{})
	if !hasCode(err, CodeInvalidState) {
		t.Fatalf("read after Close = %v; want invalid_state", err)
	}
	_, err = eng.CreateEntity(context.Background(), CreateEntityRequest{Type: "requirement", Title: "Closed"})
	if !hasCode(err, CodeInvalidState) {
		t.Fatalf("write after Close = %v; want invalid_state", err)
	}
	if err := eng.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestCloseWaitsForAdmittedRefresh(t *testing.T) {
	eng, err := Open(context.Background(), Options{Root: newTestRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	refresh := eng.lock.onAcquire
	eng.lock.onAcquire = func() error {
		close(entered)
		<-release
		return refresh()
	}
	readDone := make(chan error, 1)
	go func() {
		_, _, err := eng.ListEntities(context.Background(), ListEntitiesRequest{})
		readDone <- err
	}()
	<-entered
	closeDone := make(chan error, 1)
	go func() { closeDone <- eng.Close() }()
	select {
	case err := <-closeDone:
		close(release)
		<-readDone
		t.Fatalf("Close returned before admitted refresh finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-readDone; err != nil {
		t.Fatalf("admitted read: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPhaseSelectionPropagatesIndexFailures(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{idx: idx}
	_, err = eng.collectPlanPhases("PLN-001")
	if !hasCode(err, CodeRuntime) {
		t.Errorf("phase collection = %v; want runtime error", err)
	}
	_, err = eng.buildPhaseInfoMap(map[string]bool{"PHS-001": true})
	if !hasCode(err, CodeRuntime) {
		t.Errorf("phase state lookup = %v; want runtime error", err)
	}
	_, err = eng.buildPredecessorMap(map[string]bool{"PHS-001": true})
	if !hasCode(err, CodeRuntime) {
		t.Errorf("predecessor lookup = %v; want runtime error", err)
	}
}

func TestPhaseNextDoesNotActivateWhenRelationsCannotBeRead(t *testing.T) {
	root := newTestRoot(t)
	eng, err := Open(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	for _, req := range []CreateEntityRequest{
		{ID: "PLN-001", Type: "plan", Title: "Plan", Status: "active"},
		{ID: "PHS-001", Type: "phase", Title: "Phase"},
	} {
		if _, err := eng.CreateEntity(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eng.AddRelation(ctx, AddRelationRequest{From: "PHS-001", To: "PLN-001", Type: "belongs_to"}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("DROP TABLE relations"); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.PhaseNext(ctx, PhaseNextRequest{Activate: true}); !hasCode(err, CodeRuntime) {
		t.Fatalf("PhaseNext = %v; want runtime error", err)
	}
	phase, err := eng.GetEntity(ctx, "PHS-001")
	if err != nil {
		t.Fatal(err)
	}
	if phase.Status != "draft" {
		t.Fatalf("phase activated after relation read failure: %s", phase.Status)
	}
}

func TestRawQueryCannotMutateIndex(t *testing.T) {
	eng, err := Open(context.Background(), Options{Root: newTestRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	if _, err := eng.CreateEntity(ctx, CreateEntityRequest{ID: "REQ-001", Type: "requirement", Title: "Original"}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"DELETE FROM entities",
		"UPDATE entities SET title = 'Changed' RETURNING id",
		"INSERT INTO _meta(key, value) VALUES ('injected', 'yes')",
		"DROP TABLE entities",
		"SELECT 1; DELETE FROM entities",
		"PRAGMA query_only=OFF; DELETE FROM entities",
		"ATTACH DATABASE '" + filepath.Join(eng.Root(), "graph.db") + "' AS writable; DELETE FROM writable.entities",
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := eng.RawQuery(ctx, query); err == nil {
				t.Fatal("mutating SQL accepted")
			}
			result, err := eng.RawQuery(ctx, "SELECT title FROM entities WHERE id = 'REQ-001'")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Rows) != 1 || result.Rows[0]["title"] != "Original" {
				t.Fatalf("index changed: %+v", result.Rows)
			}
		})
	}
}

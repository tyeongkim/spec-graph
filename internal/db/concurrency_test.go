package db

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentWrites_FileBacked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrent.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const goroutines = 50

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				errs <- err
				return
			}
			if _, err := tx.Exec(
				`INSERT INTO entities (id, type, title, status) VALUES (?, ?, ?, ?)`,
				formatID(idx), "requirement", "concurrent", "draft",
			); err != nil {
				_ = tx.Rollback()
				errs <- err
				return
			}
			if err := tx.Commit(); err != nil {
				errs <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	var collected []error
	for e := range errs {
		collected = append(collected, e)
	}
	if len(collected) > 0 {
		t.Fatalf("got %d errors during concurrent writes; first: %v", len(collected), collected[0])
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entities`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != goroutines {
		t.Errorf("got %d rows; want %d", count, goroutines)
	}
}

func TestConcurrentReadersDuringWrites(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rw.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO entities (id, type, title, status) VALUES ('REQ-000', 'requirement', 'seed', 'active')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const writers = 20
	const readers = 20

	var wg sync.WaitGroup
	errs := make(chan error, writers+readers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if _, err := db.Exec(
				`INSERT INTO entities (id, type, title, status) VALUES (?, ?, ?, ?)`,
				formatID(idx+1), "requirement", "rw", "draft",
			); err != nil {
				errs <- err
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := db.Query(`SELECT id FROM entities`)
			if err != nil {
				errs <- err
				return
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					errs <- err
					rows.Close()
					return
				}
			}
			rows.Close()
		}()
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Errorf("error during concurrent read/write: %v", e)
	}
}

func TestPragmaSynchronous(t *testing.T) {
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB: %v", err)
	}
	defer db.Close()

	var sync int
	if err := db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("query synchronous: %v", err)
	}
	if sync != 1 {
		t.Errorf("PRAGMA synchronous = %d; want 1 (NORMAL)", sync)
	}
}

func TestPragmaBusyTimeout(t *testing.T) {
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB: %v", err)
	}
	defer db.Close()

	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if timeout != 30000 {
		t.Errorf("PRAGMA busy_timeout = %d; want 30000", timeout)
	}
}

func TestConnectionPoolPinnedToOne(t *testing.T) {
	db, err := OpenMemoryDB()
	if err != nil {
		t.Fatalf("OpenMemoryDB: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d; want 1", stats.MaxOpenConnections)
	}
}

func formatID(i int) string {
	return fmt.Sprintf("REQ-%03d", i)
}

package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}

func OpenDB(path string) (*sql.DB, error) {
	return openDB(path)
}

func OpenMemoryDB() (*sql.DB, error) {
	return openDB(":memory:")
}

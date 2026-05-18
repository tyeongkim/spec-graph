package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// connectionPragmas are applied to every connection via the _pragma DSN
// parameter so they take effect for in-memory and file-backed databases alike.
//
// synchronous=NORMAL is safe with WAL and avoids per-commit fsync.
// busy_timeout=30000 absorbs cross-process writer contention.
// foreign_keys=ON keeps referential integrity on every connection (PRAGMA is
// connection-scoped and the default is OFF).
// temp_store=MEMORY keeps temp tables/indices off disk for short transactions.
var connectionPragmas = []string{
	"busy_timeout(30000)",
	"foreign_keys(1)",
	"synchronous(NORMAL)",
	"temp_store(2)",
}

func buildDSN(path string) string {
	q := url.Values{}
	for _, p := range connectionPragmas {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")

	if path == ":memory:" {
		return ":memory:?" + q.Encode()
	}
	return "file:" + path + "?" + q.Encode()
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}

	// SQLite is a single-writer database. Pinning the pool to a single
	// connection serialises in-process writes cleanly and prevents the
	// classic deferred-transaction lock-upgrade deadlock between two Go
	// connections racing for the writer slot.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	return db, nil
}

func OpenDB(path string) (*sql.DB, error) {
	return openDB(buildDSN(path))
}

func OpenMemoryDB() (*sql.DB, error) {
	return openDB(buildDSN(":memory:"))
}

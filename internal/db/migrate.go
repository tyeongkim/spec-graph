package db

import (
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/001_init.sql
var initSQL embed.FS

func Migrate(db *sql.DB) error {
	data, err := initSQL.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	if _, err := db.Exec(string(data)); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	return nil
}

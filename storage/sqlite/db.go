// Package sqlite provides the local SQLite-backed persistence for a case.
// It implements sdk.Repository. Opening a database applies migrations and sets
// pragmas tuned for a single-process embedded workload.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bctx/bctx/storage/migrations"
)

// Open opens (creating if needed) the SQLite database at path, applies
// migrations, and returns a handle. No network access occurs.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// Single writer; keep a modest pool.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	if err := migrations.Apply(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return db, nil
}

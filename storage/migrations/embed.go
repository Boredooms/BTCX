// Package migrations embeds the SQL migration files and applies them to a
// case database. Schema changes are always applied through migrations.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Apply runs all pending migrations in lexical order and records applied
// versions in schema_migrations.
func Apply(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TEXT NOT NULL DEFAULT (datetime('now'))
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists string
		err := db.QueryRow(`SELECT version FROM schema_migrations WHERE version = ?`, name).Scan(&exists)
		if err == nil {
			continue // already applied
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", name, err)
		}

		sqlBytes, rerr := files.ReadFile(name)
		if rerr != nil {
			return fmt.Errorf("read migration %s: %w", name, rerr)
		}

		tx, berr := db.Begin()
		if berr != nil {
			return fmt.Errorf("begin migration %s: %w", name, berr)
		}
		for _, stmt := range splitStatements(string(sqlBytes)) {
			if _, eerr := tx.Exec(stmt); eerr != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %s: %w", name, eerr)
			}
		}
		if _, eerr := tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, name); eerr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, eerr)
		}
		if cerr := tx.Commit(); cerr != nil {
			return fmt.Errorf("commit migration %s: %w", name, cerr)
		}
	}
	return nil
}

// splitStatements breaks a migration file into individual SQL statements.
// go-sqlite3's Exec does not reliably run every statement of a multi-statement
// string inside an explicit transaction, so each statement is executed
// separately. Line comments (`-- ...`) are stripped and statements are split on
// semicolons that fall outside single-quoted string literals.
func splitStatements(sql string) []string {
	var stmts []string
	var b strings.Builder
	inString := false

	for _, line := range strings.Split(sql, "\n") {
		// Strip a trailing line comment that is not inside a string literal.
		if !inString {
			if idx := indexLineComment(line); idx >= 0 {
				line = line[:idx]
			}
		}
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == '\'' {
				inString = !inString
			}
			if c == ';' && !inString {
				stmt := strings.TrimSpace(b.String())
				if stmt != "" {
					stmts = append(stmts, stmt)
				}
				b.Reset()
				continue
			}
			b.WriteByte(c)
		}
		b.WriteByte('\n')
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

// indexLineComment returns the index of a `--` line comment that is not inside
// a single-quoted string literal, or -1 if there is none.
func indexLineComment(line string) int {
	inString := false
	for i := 0; i < len(line); i++ {
		if line[i] == '\'' {
			inString = !inString
			continue
		}
		if !inString && line[i] == '-' && i+1 < len(line) && line[i+1] == '-' {
			return i
		}
	}
	return -1
}

package registry

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path, creates parent
// directories as needed, enables WAL mode, and runs all schema migrations.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("registry.Open: create directories: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, fmt.Errorf("registry.Open: open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite handles one writer at a time

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("registry.Open: migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS raw_items (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			source       TEXT NOT NULL,
			url          TEXT UNIQUE NOT NULL,
			title        TEXT,
			body         TEXT,
			score        INTEGER DEFAULT 0,
			author       TEXT,
			created_at   DATETIME,
			gathered_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			fed_to_claude BOOLEAN DEFAULT FALSE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_raw_items_fed    ON raw_items(fed_to_claude)`,
		`CREATE INDEX IF NOT EXISTS idx_raw_items_source ON raw_items(source)`,

		`CREATE TABLE IF NOT EXISTS build_queue (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT NOT NULL,
			problem         TEXT NOT NULL,
			source_url      TEXT,
			solution        TEXT NOT NULL,
			language        TEXT NOT NULL,
			files           TEXT NOT NULL,
			estimated_lines INTEGER,
			status          TEXT DEFAULT 'queued',
			attempts        INTEGER DEFAULT 0,
			queued_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at      DATETIME,
			shipped_at      DATETIME,
			error_log       TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_build_queue_status ON build_queue(status)`,

		`CREATE TABLE IF NOT EXISTS repos (
			name            TEXT PRIMARY KEY,
			language        TEXT,
			problem         TEXT,
			source_url      TEXT,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_maintained DATETIME,
			version         TEXT DEFAULT 'v0.1.0',
			lines_of_code   INTEGER,
			has_tests       BOOLEAN DEFAULT FALSE,
			tests_pass      BOOLEAN DEFAULT FALSE,
			github_pushed   BOOLEAN DEFAULT FALSE,
			github_push_at  DATETIME
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(40, len(stmt))], err)
		}
	}

	// Additive migrations — add columns for idea-engine integration.
	// These use a helper that silently ignores "duplicate column" errors
	// so they're safe to run on both new and existing databases.
	alterations := []string{
		`ALTER TABLE build_queue ADD COLUMN source_papers TEXT`,
		`ALTER TABLE build_queue ADD COLUMN source_repos TEXT`,
		`ALTER TABLE build_queue ADD COLUMN market_analysis TEXT`,
	}
	for _, stmt := range alterations {
		if _, err := db.Exec(stmt); err != nil {
			// SQLite returns "duplicate column name" if the column already exists.
			// This is expected on subsequent opens — ignore it.
			if !isDuplicateColumn(err) {
				return fmt.Errorf("alter: %w", err)
			}
		}
	}

	return nil
}

// isDuplicateColumn returns true if the error indicates an ALTER TABLE
// tried to add a column that already exists.
func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column name")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

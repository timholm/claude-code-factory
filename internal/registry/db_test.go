package registry_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := registry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	tables := []string{"raw_items", "build_queue", "repos"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %q not found in database", table)
		} else if err != nil {
			t.Errorf("querying for table %q: %v", table, err)
		}
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "registry.db")

	db, err := registry.Open(nested)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Errorf("expected DB file to exist at %s", nested)
	}
}

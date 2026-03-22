package registry

import (
	"database/sql"
	"fmt"
	"strings"
)

// Registry wraps a SQLite database handle and provides domain-level operations.
type Registry struct {
	DB *sql.DB
}

// RawItem represents a single row from the raw_items table.
type RawItem struct {
	ID     int64
	Source string
	URL    string
	Title  string
	Body   string
	Score  int
	Author string
}

// InsertRawItem inserts a new raw item, silently ignoring duplicate URLs.
func (r *Registry) InsertRawItem(source, url, title, body string, score int, author string) error {
	const q = `INSERT OR IGNORE INTO raw_items (source, url, title, body, score, author)
	            VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := r.DB.Exec(q, source, url, title, body, score, author); err != nil {
		return fmt.Errorf("registry.InsertRawItem: %w", err)
	}
	return nil
}

// GetUnfedItems returns all rows where fed_to_claude = FALSE, ordered by id.
func (r *Registry) GetUnfedItems() ([]RawItem, error) {
	const q = `SELECT id, source, url, title, body, score, author
	           FROM raw_items
	           WHERE fed_to_claude = FALSE
	           ORDER BY id`

	rows, err := r.DB.Query(q)
	if err != nil {
		return nil, fmt.Errorf("registry.GetUnfedItems: %w", err)
	}
	defer rows.Close()

	var items []RawItem
	for rows.Next() {
		var it RawItem
		if err := rows.Scan(&it.ID, &it.Source, &it.URL, &it.Title, &it.Body, &it.Score, &it.Author); err != nil {
			return nil, fmt.Errorf("registry.GetUnfedItems scan: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registry.GetUnfedItems rows: %w", err)
	}
	return items, nil
}

// MarkItemsFed sets fed_to_claude = TRUE for all given IDs inside a transaction.
func (r *Registry) MarkItemsFed(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("registry.MarkItemsFed begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	q := fmt.Sprintf(`UPDATE raw_items SET fed_to_claude = TRUE WHERE id IN (%s)`, placeholders)

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	if _, err := tx.Exec(q, args...); err != nil {
		return fmt.Errorf("registry.MarkItemsFed exec: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("registry.MarkItemsFed commit: %w", err)
	}
	return nil
}

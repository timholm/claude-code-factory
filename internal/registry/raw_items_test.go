package registry

import (
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *Registry {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Registry{DB: db}
}

func TestInsertRawItem(t *testing.T) {
	r := testDB(t)

	// Insert one item — should succeed.
	if err := r.InsertRawItem("hn", "https://example.com/1", "Title 1", "Body 1", 42, "alice"); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Insert duplicate URL — should not error (INSERT OR IGNORE).
	if err := r.InsertRawItem("hn", "https://example.com/1", "Title 1 dupe", "Body 1 dupe", 99, "bob"); err != nil {
		t.Fatalf("duplicate insert: %v", err)
	}
}

func TestGetUnfedItems(t *testing.T) {
	r := testDB(t)

	if err := r.InsertRawItem("hn", "https://example.com/1", "T1", "B1", 10, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := r.InsertRawItem("hn", "https://example.com/2", "T2", "B2", 20, "bob"); err != nil {
		t.Fatal(err)
	}

	items, err := r.GetUnfedItems()
	if err != nil {
		t.Fatalf("GetUnfedItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestMarkFed(t *testing.T) {
	r := testDB(t)

	if err := r.InsertRawItem("hn", "https://example.com/1", "T1", "B1", 10, "alice"); err != nil {
		t.Fatal(err)
	}

	items, err := r.GetUnfedItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 unfed item, got %d", len(items))
	}

	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}

	if err := r.MarkItemsFed(ids); err != nil {
		t.Fatalf("MarkItemsFed: %v", err)
	}

	items, err = r.GetUnfedItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 unfed items after marking fed, got %d", len(items))
	}
}

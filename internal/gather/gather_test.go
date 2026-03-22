package gather_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/timholmquist/claude-code-factory/internal/gather"
	"github.com/timholmquist/claude-code-factory/internal/registry"
)

// fakeScraper is a test double that returns a fixed set of items.
type fakeScraper struct {
	items []gather.Item
}

func (f *fakeScraper) Name() string { return "fake" }
func (f *fakeScraper) Scrape(_ context.Context) ([]gather.Item, error) {
	return f.items, nil
}

// testRegistry opens a throw-away SQLite DB in t.TempDir.
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	db, err := registry.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &registry.Registry{DB: db}
}

func TestGatherRunsScrapersAndStores(t *testing.T) {
	reg := testRegistry(t)

	items := []gather.Item{
		{Source: "fake", URL: "https://example.com/1", Title: "Post 1", Body: "Body 1", Score: 10, Author: "alice"},
		{Source: "fake", URL: "https://example.com/2", Title: "Post 2", Body: "Body 2", Score: 20, Author: "bob"},
	}

	fs := &fakeScraper{items: items}

	count, err := gather.Run(context.Background(), reg, []gather.Scraper{fs})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}

	// Verify items are actually in the DB.
	stored, err := reg.GetUnfedItems()
	if err != nil {
		t.Fatalf("GetUnfedItems: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 stored items, got %d", len(stored))
	}
}

func TestGatherSkipsFailingScraper(t *testing.T) {
	reg := testRegistry(t)

	// goodScraper contributes one item.
	good := &fakeScraper{items: []gather.Item{
		{Source: "good", URL: "https://example.com/good", Title: "Good", Body: "", Score: 5, Author: "tim"},
	}}

	// errorScraper always fails.
	bad := &errorScraper{}

	count, err := gather.Run(context.Background(), reg, []gather.Scraper{bad, good})
	// An error is returned (from the failing scraper) but processing continues.
	if err == nil {
		t.Fatal("expected non-nil error from failing scraper")
	}
	// The good scraper's item should still be stored.
	if count != 1 {
		t.Fatalf("expected count=1 (good scraper), got %d", count)
	}

	stored, err2 := reg.GetUnfedItems()
	if err2 != nil {
		t.Fatalf("GetUnfedItems: %v", err2)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored item, got %d", len(stored))
	}
}

// errorScraper always returns an error.
type errorScraper struct{}

func (e *errorScraper) Name() string { return "error" }
func (e *errorScraper) Scrape(_ context.Context) ([]gather.Item, error) {
	return nil, fmt.Errorf("scraper broke")
}

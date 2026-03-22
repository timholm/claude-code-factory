package gather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHNScraper_Name(t *testing.T) {
	s := &HNScraper{}
	if got := s.Name(); got != "hn" {
		t.Fatalf("Name() = %q, want %q", got, "hn")
	}
}

func TestHNScraper_Scrape(t *testing.T) {
	payload := map[string]any{
		"hits": []map[string]any{
			{
				"objectID": "12345",
				"title":    "Claude Code is amazing",
				"url":      "https://example.com/claude-code",
				"author":   "alice",
				"points":   42,
			},
			{
				"objectID": "67890",
				"title":    "Ask HN: Anyone using Claude Code?",
				"url":      "", // no URL → should fall back to HN link
				"author":   "bob",
				"points":   10,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the query path looks right.
		if r.URL.Path != "/search_by_date" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	scraper := &HNScraper{BaseURL: srv.URL}
	items, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First item: has a real URL.
	first := items[0]
	if first.Source != "hn" {
		t.Errorf("items[0].Source = %q, want %q", first.Source, "hn")
	}
	if first.URL != "https://example.com/claude-code" {
		t.Errorf("items[0].URL = %q, want external URL", first.URL)
	}
	if first.Title != "Claude Code is amazing" {
		t.Errorf("items[0].Title = %q", first.Title)
	}
	if first.Author != "alice" {
		t.Errorf("items[0].Author = %q, want alice", first.Author)
	}
	if first.Score != 42 {
		t.Errorf("items[0].Score = %d, want 42", first.Score)
	}

	// Second item: empty URL → must fall back to HN item link.
	second := items[1]
	wantURL := "https://news.ycombinator.com/item?id=67890"
	if second.URL != wantURL {
		t.Errorf("items[1].URL = %q, want %q", second.URL, wantURL)
	}
	if second.Score != 10 {
		t.Errorf("items[1].Score = %d, want 10", second.Score)
	}
}

func TestHNScraper_Scrape_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	scraper := &HNScraper{BaseURL: srv.URL}
	_, err := scraper.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestHNScraper_Scrape_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request should never reach here (context already cancelled).
		json.NewEncoder(w).Encode(map[string]any{"hits": []any{}})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	scraper := &HNScraper{BaseURL: srv.URL}
	_, err := scraper.Scrape(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestHNScraper_ImplementsScraper is a compile-time assertion.
var _ Scraper = (*HNScraper)(nil)

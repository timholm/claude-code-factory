package gather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const mockGitHubIssuesResponse = `[
  {
    "html_url": "https://github.com/anthropics/claude-code/issues/1",
    "title": "Feature request: add streaming support",
    "body": "It would be great to have streaming support for long responses.",
    "reactions": {"total_count": 42},
    "user": {"login": "alice"}
  },
  {
    "html_url": "https://github.com/anthropics/claude-code/issues/2",
    "title": "Bug: crash on empty input",
    "body": "The tool crashes when given empty input.",
    "reactions": {"total_count": 7},
    "user": {"login": "bob"}
  }
]`

func TestGitHubIssuesScraper_Name(t *testing.T) {
	g := &GitHubIssuesScraper{}
	if got := g.Name(); got != "github_issue" {
		t.Errorf("Name() = %q, want %q", got, "github_issue")
	}
}

func TestGitHubIssuesScraper_Scrape(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		// Verify the correct endpoint is called
		if r.URL.Path != "/repos/anthropics/claude-code/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("state") != "open" {
			t.Errorf("expected state=open, got %s", q.Get("state"))
		}
		if q.Get("sort") != "created" {
			t.Errorf("expected sort=created, got %s", q.Get("sort"))
		}
		if q.Get("per_page") != "100" {
			t.Errorf("expected per_page=100, got %s", q.Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockGitHubIssuesResponse))
	}))
	defer ts.Close()

	g := &GitHubIssuesScraper{
		BaseURL: ts.URL,
		Token:   "test-token-xyz",
	}

	items, err := g.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Scrape() returned %d items, want 2", len(items))
	}

	// Verify Authorization header was sent
	if capturedAuth != "Bearer test-token-xyz" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token-xyz")
	}

	// Check first item
	item := items[0]
	if item.Source != "github_issue" {
		t.Errorf("Source = %q, want %q", item.Source, "github_issue")
	}
	if item.URL != "https://github.com/anthropics/claude-code/issues/1" {
		t.Errorf("URL = %q", item.URL)
	}
	if item.Title != "Feature request: add streaming support" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.Body != "It would be great to have streaming support for long responses." {
		t.Errorf("Body = %q", item.Body)
	}
	if item.Score != 42 {
		t.Errorf("Score = %d, want 42", item.Score)
	}
	if item.Author != "alice" {
		t.Errorf("Author = %q, want %q", item.Author, "alice")
	}

	// Check second item
	item2 := items[1]
	if item2.Author != "bob" {
		t.Errorf("Author = %q, want %q", item2.Author, "bob")
	}
	if item2.Score != 7 {
		t.Errorf("Score = %d, want 7", item2.Score)
	}
}

func TestGitHubIssuesScraper_NoToken(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	g := &GitHubIssuesScraper{BaseURL: ts.URL}
	_, err := g.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if capturedAuth != "" {
		t.Errorf("expected no Authorization header without token, got %q", capturedAuth)
	}
}

func TestGitHubIssuesScraper_BodyTruncation(t *testing.T) {
	longBody := make([]byte, 3000)
	for i := range longBody {
		longBody[i] = 'x'
	}
	// JSON-encode a long body issue
	mockResp := `[{"html_url":"https://example.com/1","title":"Long","body":"` +
		string(longBody) + `","reactions":{"total_count":0},"user":{"login":"user1"}}]`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResp))
	}))
	defer ts.Close()

	g := &GitHubIssuesScraper{BaseURL: ts.URL}
	items, err := g.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Body) != 2000 {
		t.Errorf("Body length = %d, want 2000", len(items[0].Body))
	}
}

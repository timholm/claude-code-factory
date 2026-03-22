package gather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const mockRedditResponse = `{
  "data": {
    "children": [
      {
        "data": {
          "title": "Claude 3.5 is amazing",
          "selftext": "I have been using it all week and it is great.",
          "permalink": "/r/ClaudeAI/comments/abc123/claude_35_is_amazing/",
          "author": "testuser",
          "score": 42
        }
      },
      {
        "data": {
          "title": "Prompt engineering tips",
          "selftext": "Here are some tips I collected.",
          "permalink": "/r/ClaudeAI/comments/def456/prompt_engineering_tips/",
          "author": "anotheruser",
          "score": 17
        }
      }
    ]
  }
}`

func TestRedditScraper_Scrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the User-Agent header is forwarded.
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing User-Agent", http.StatusBadRequest)
			return
		}
		// Only respond to /r/ClaudeAI/new.json; return 404 for others.
		if !strings.HasPrefix(r.URL.Path, "/r/ClaudeAI/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockRedditResponse))
	}))
	defer ts.Close()

	rs := &RedditScraper{
		BaseURL:   ts.URL,
		UserAgent: "test-agent/1.0",
	}

	items, err := rs.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape returned unexpected error: %v", err)
	}

	// ClaudeAI returns 2 items; ChatGPTPro and LocalLLaMA return 404 (logged, skipped).
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0]
	if first.Source != "reddit" {
		t.Errorf("Source = %q, want %q", first.Source, "reddit")
	}
	if first.Title != "Claude 3.5 is amazing" {
		t.Errorf("Title = %q, want %q", first.Title, "Claude 3.5 is amazing")
	}
	if first.Author != "testuser" {
		t.Errorf("Author = %q, want %q", first.Author, "testuser")
	}
	if first.Score != 42 {
		t.Errorf("Score = %d, want 42", first.Score)
	}
	// URL must be prepended with the real Reddit domain, not the test server URL.
	wantURL := "https://www.reddit.com/r/ClaudeAI/comments/abc123/claude_35_is_amazing/"
	if first.URL != wantURL {
		t.Errorf("URL = %q, want %q", first.URL, wantURL)
	}
}

func TestRedditScraper_Name(t *testing.T) {
	rs := &RedditScraper{}
	if rs.Name() != "reddit" {
		t.Errorf("Name() = %q, want %q", rs.Name(), "reddit")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		max    int
		expect string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.expect {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expect)
		}
	}
}

func TestRedditScraper_SelfTextTruncation(t *testing.T) {
	longText := strings.Repeat("a", 3000)
	body := `{"data":{"children":[{"data":{"title":"T","selftext":"` + longText + `","permalink":"/r/ClaudeAI/comments/x/t/","author":"u","score":1}}]}}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer ts.Close()

	rs := &RedditScraper{BaseURL: ts.URL, UserAgent: "test/1.0"}
	items, err := rs.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one item")
	}
	if len([]rune(items[0].Body)) != 2000 {
		t.Errorf("Body length = %d, want 2000", len([]rune(items[0].Body)))
	}
}

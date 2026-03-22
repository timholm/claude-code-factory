package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const redditDefaultBaseURL = "https://www.reddit.com"

// RedditScraper fetches new posts from a set of subreddits via the Reddit JSON API.
type RedditScraper struct {
	BaseURL   string // override for testing; default: https://www.reddit.com
	UserAgent string // Reddit requires a User-Agent
}

func (rs *RedditScraper) Name() string { return "reddit" }

// Scrape fetches the newest 50 posts from each target subreddit and returns
// them as Items. If a single subreddit request fails the error is logged and
// the remaining subreddits are still fetched.
func (rs *RedditScraper) Scrape(ctx context.Context) ([]Item, error) {
	baseURL := rs.BaseURL
	if baseURL == "" {
		baseURL = redditDefaultBaseURL
	}

	subreddits := []string{"ClaudeAI", "ChatGPTPro", "LocalLLaMA"}

	var items []Item
	for _, sub := range subreddits {
		got, err := rs.scrapeSubreddit(ctx, baseURL, sub)
		if err != nil {
			log.Printf("reddit: subreddit %q failed: %v", sub, err)
			continue
		}
		items = append(items, got...)
	}
	return items, nil
}

// redditListing is the top-level shape returned by /r/<sub>/new.json.
type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				Title     string `json:"title"`
				Selftext  string `json:"selftext"`
				Permalink string `json:"permalink"`
				Author    string `json:"author"`
				Score     int    `json:"score"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func (rs *RedditScraper) scrapeSubreddit(ctx context.Context, baseURL, sub string) ([]Item, error) {
	url := fmt.Sprintf("%s/r/%s/new.json?limit=50", baseURL, sub)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", rs.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	var listing redditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("decode JSON from %s: %w", url, err)
	}

	items := make([]Item, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		d := child.Data
		items = append(items, Item{
			Source: "reddit",
			URL:    redditDefaultBaseURL + d.Permalink,
			Title:  d.Title,
			Body:   truncate(d.Selftext, 2000),
			Score:  d.Score,
			Author: d.Author,
		})
	}
	return items, nil
}

// truncate shortens s to at most maxLen runes.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GitHubIssuesScraper fetches open issues from the anthropics/claude-code repo.
type GitHubIssuesScraper struct {
	BaseURL string // override for testing; default: https://api.github.com
	Token   string
}

// Name returns the source identifier for items produced by this scraper.
func (g *GitHubIssuesScraper) Name() string { return "github_issue" }

// Scrape fetches open GitHub issues and returns them as Items.
func (g *GitHubIssuesScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := g.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}

	url := base + "/repos/anthropics/claude-code/issues?state=open&sort=created&per_page=100"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github scraper: build request: %w", err)
	}

	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github scraper: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github scraper: unexpected status %d", resp.StatusCode)
	}

	var raw []struct {
		HTMLURL   string `json:"html_url"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Reactions struct {
			TotalCount int `json:"total_count"`
		} `json:"reactions"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github scraper: decode response: %w", err)
	}

	items := make([]Item, 0, len(raw))
	for _, r := range raw {
		items = append(items, Item{
			Source: g.Name(),
			URL:    r.HTMLURL,
			Title:  r.Title,
			Body:   truncate(r.Body, 2000),
			Score:  r.Reactions.TotalCount,
			Author: r.User.Login,
		})
	}
	return items, nil
}


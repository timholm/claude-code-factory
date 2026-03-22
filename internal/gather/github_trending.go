package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GitHubTrendingScraper finds trending AI/ML repos and new innovative projects.
type GitHubTrendingScraper struct {
	BaseURL string
	Token   string
}

func (g *GitHubTrendingScraper) Name() string { return "github_trending" }

func (g *GitHubTrendingScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := g.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}

	queries := []string{
		"ai agent framework created:>2026-03-01",
		"mcp server created:>2026-03-01",
		"LLM tool created:>2026-03-01",
		"agentic workflow created:>2026-03-01",
		"AI developer tools created:>2026-03-01",
		"autonomous agent created:>2026-03-01",
		"AI infrastructure created:>2026-03-01",
		"edge AI created:>2026-03-01",
		"multimodal AI created:>2026-03-01",
		"AI code generation created:>2026-03-01",
	}

	var allItems []Item
	for _, q := range queries {
		items, err := g.search(ctx, base, q)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

func (g *GitHubTrendingScraper) search(ctx context.Context, base, query string) ([]Item, error) {
	url := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc&per_page=20", base, query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if g.Token != "" {
		req.Header.Set("Authorization", "token "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github search returned %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			HTMLURL     string `json:"html_url"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			Language    string `json:"language"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(result.Items))
	for _, repo := range result.Items {
		desc := repo.Description
		if repo.Language != "" {
			desc = fmt.Sprintf("[%s] %s", repo.Language, desc)
		}
		items = append(items, Item{
			Source: "github_trending",
			URL:    repo.HTMLURL,
			Title:  repo.FullName,
			Body:   desc,
			Score:  repo.Stars,
			Author: repo.Owner.Login,
		})
	}
	return items, nil
}

package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// YCombinatorScraper finds recent AI startup launches from Y Combinator.
type YCombinatorScraper struct {
	BaseURL string
}

func (y *YCombinatorScraper) Name() string { return "ycombinator" }

func (y *YCombinatorScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := y.BaseURL
	if base == "" {
		base = "https://hn.algolia.com/api/v1"
	}

	// Search for YC launches and AI startup announcements
	queries := []string{
		"Launch HN AI",
		"Show HN AI agent",
		"Show HN LLM",
		"YC AI startup",
		"Show HN developer tools AI",
		"Show HN open source AI",
	}

	var allItems []Item
	for _, q := range queries {
		url := fmt.Sprintf("%s/search_by_date?query=%s&tags=show_hn&hitsPerPage=20", base, q)
		items, err := y.fetch(ctx, url)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

func (y *YCombinatorScraper) fetch(ctx context.Context, url string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Hits []struct {
			ObjectID string `json:"objectID"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Author   string `json:"author"`
			Points   int    `json:"points"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(result.Hits))
	for _, hit := range result.Hits {
		itemURL := hit.URL
		if itemURL == "" {
			itemURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		}
		items = append(items, Item{
			Source: "ycombinator",
			URL:    itemURL,
			Title:  hit.Title,
			Score:  hit.Points,
			Author: hit.Author,
		})
	}
	return items, nil
}

package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const hnDefaultBaseURL = "https://hn.algolia.com/api/v1"

// HNScraper fetches stories from the Hacker News Algolia API.
type HNScraper struct {
	// BaseURL can be overridden in tests; defaults to hnDefaultBaseURL.
	BaseURL string
}

func (h *HNScraper) Name() string { return "hn" }

// Scrape fetches the 50 most-recent HN stories mentioning "claude code".
func (h *HNScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := h.BaseURL
	if base == "" {
		base = hnDefaultBaseURL
	}

	url := fmt.Sprintf("%s/search_by_date?query=claude+code&tags=story&hitsPerPage=50", base)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hn: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hn: http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hn: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Hits []struct {
			ObjectID string `json:"objectID"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Author   string `json:"author"`
			Points   int    `json:"points"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("hn: decode json: %w", err)
	}

	items := make([]Item, 0, len(payload.Hits))
	for _, hit := range payload.Hits {
		itemURL := hit.URL
		if itemURL == "" {
			itemURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		}
		items = append(items, Item{
			Source: "hn",
			URL:    itemURL,
			Title:  hit.Title,
			Author: hit.Author,
			Score:  hit.Points,
		})
	}

	return items, nil
}

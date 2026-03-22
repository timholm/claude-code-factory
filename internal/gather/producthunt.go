package gather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ProductHuntScraper scrapes Product Hunt for new AI products launched today.
type ProductHuntScraper struct {
	BaseURL string
}

func (p *ProductHuntScraper) Name() string { return "producthunt" }

func (p *ProductHuntScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://www.producthunt.com"
	}

	// PH has a public JSON endpoint for today's posts
	url := fmt.Sprintf("%s/frontend/graphql", base)

	// Use the public feed endpoint instead
	url = fmt.Sprintf("%s/api/v1/posts/all.json?sort_by=votes_count&order=desc&per_page=50&search[topic]=artificial-intelligence", base)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("producthunt returned %d", resp.StatusCode)
	}

	var result struct {
		Posts []struct {
			Name        string `json:"name"`
			Tagline     string `json:"tagline"`
			URL         string `json:"discussion_url"`
			VotesCount  int    `json:"votes_count"`
			MakerInside bool   `json:"maker_inside"`
		} `json:"posts"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(result.Posts))
	for _, post := range result.Posts {
		items = append(items, Item{
			Source: "producthunt",
			URL:    post.URL,
			Title:  post.Name,
			Body:   post.Tagline,
			Score:  post.VotesCount,
		})
	}
	return items, nil
}

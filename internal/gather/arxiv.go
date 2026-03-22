package gather

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
)

// ArxivScraper scrapes arXiv for recent AI/ML/LLM research papers.
type ArxivScraper struct {
	BaseURL string
}

func (a *ArxivScraper) Name() string { return "arxiv" }

func (a *ArxivScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := a.BaseURL
	if base == "" {
		base = "https://export.arxiv.org/api"
	}

	queries := []string{
		"cat:cs.AI+AND+abs:agent",
		"cat:cs.CL+AND+abs:code+generation",
		"cat:cs.SE+AND+abs:LLM",
		"abs:model+context+protocol",
		"abs:agentic+workflow",
		"abs:AI+developer+tools",
	}

	var allItems []Item
	for _, q := range queries {
		url := fmt.Sprintf("%s/query?search_query=%s&sortBy=submittedDate&sortOrder=descending&max_results=20", base, q)

		items, err := a.fetch(ctx, url)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

func (a *ArxivScraper) fetch(ctx context.Context, url string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var feed struct {
		Entries []struct {
			Title   string `xml:"title"`
			Summary string `xml:"summary"`
			ID      string `xml:"id"`
			Authors []struct {
				Name string `xml:"name"`
			} `xml:"author"`
		} `xml:"entry"`
	}

	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		author := ""
		if len(e.Authors) > 0 {
			author = e.Authors[0].Name
		}
		items = append(items, Item{
			Source: "arxiv",
			URL:    e.ID,
			Title:  e.Title,
			Body:   truncate(e.Summary, 2000),
			Author: author,
			Score:  0,
		})
	}
	return items, nil
}

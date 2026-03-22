package gather

import "context"

// Item is a single piece of content returned by a Scraper.
type Item struct {
	Source string
	URL    string
	Title  string
	Body   string
	Score  int
	Author string
}

// Scraper is the interface every source (HN, Reddit, …) must satisfy.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context) ([]Item, error)
}

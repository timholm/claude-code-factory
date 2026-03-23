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

// Scraper is the interface every source must satisfy.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context) ([]Item, error)
}

// truncate shortens s to at most maxLen runes.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

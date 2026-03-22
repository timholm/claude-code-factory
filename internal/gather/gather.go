package gather

import (
	"context"
	"fmt"
	"log"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

// Run calls every scraper in order, inserts each item into the registry, and
// returns the total number of items successfully stored. A scraper failure is
// logged and skipped so that remaining scrapers still run.
func Run(ctx context.Context, reg *registry.Registry, scrapers []Scraper) (int, error) {
	total := 0
	var firstErr error

	for _, s := range scrapers {
		items, err := s.Scrape(ctx)
		if err != nil {
			log.Printf("gather: scraper %q failed: %v", s.Name(), err)
			if firstErr == nil {
				firstErr = fmt.Errorf("scraper %q: %w", s.Name(), err)
			}
			continue
		}

		for _, item := range items {
			if err := reg.InsertRawItem(item.Source, item.URL, item.Title, item.Body, item.Score, item.Author); err != nil {
				log.Printf("gather: insert item from %q failed: %v", s.Name(), err)
				if firstErr == nil {
					firstErr = fmt.Errorf("insert item from scraper %q: %w", s.Name(), err)
				}
				continue
			}
			total++
		}
	}

	return total, firstErr
}

package gather

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ArxivScraper scrapes arXiv for recent research papers on LLM tooling,
// infrastructure, and techniques that can be turned into paid developer tools.
type ArxivScraper struct {
	BaseURL string
}

func (a *ArxivScraper) Name() string { return "arxiv" }

// queries returns arXiv API search queries targeting monetizable LLM tooling research.
// Each query is a (label, search_query) pair.
func (a *ArxivScraper) queries() []struct{ label, query string } {
	return []struct{ label, query string }{
		// Inference & serving — the money layer
		{"llm-inference", "abs:LLM+AND+abs:inference+AND+abs:optimization"},
		{"llm-serving", "abs:language+model+AND+abs:serving"},
		{"speculative-decoding", "abs:speculative+decoding"},
		{"kv-cache", "abs:KV+cache+AND+abs:language+model"},
		{"quantization", "abs:quantization+AND+abs:large+language+model"},
		{"model-compression", "abs:model+compression+AND+abs:LLM"},

		// RAG & retrieval — enterprise cash cow
		{"rag", "abs:retrieval+augmented+generation"},
		{"dense-retrieval", "abs:dense+retrieval+AND+abs:language+model"},
		{"embedding", "abs:embedding+AND+abs:LLM+AND+abs:search"},

		// Evaluation & benchmarking — every team needs this
		{"llm-evaluation", "abs:LLM+AND+abs:evaluation+AND+abs:benchmark"},
		{"llm-testing", "abs:language+model+AND+abs:testing"},
		{"hallucination-detection", "abs:hallucination+AND+abs:detection+AND+abs:language+model"},

		// Agents & tool use — hottest market
		{"llm-agents", "cat:cs.AI+AND+abs:agent+AND+abs:tool+use"},
		{"agent-planning", "abs:LLM+AND+abs:planning+AND+abs:agent"},
		{"multi-agent", "abs:multi-agent+AND+abs:language+model"},
		{"code-agent", "abs:code+generation+AND+abs:agent"},

		// Fine-tuning & alignment — every enterprise buyer
		{"fine-tuning", "abs:fine-tuning+AND+abs:large+language+model"},
		{"rlhf", "abs:reinforcement+learning+AND+abs:human+feedback"},
		{"dpo", "abs:direct+preference+optimization"},
		{"lora", "abs:LoRA+AND+abs:language+model"},

		// Safety & guardrails — compliance money
		{"llm-safety", "abs:safety+AND+abs:large+language+model"},
		{"prompt-injection", "abs:prompt+injection"},
		{"llm-guardrails", "abs:guardrails+AND+abs:language+model"},
		{"output-filtering", "abs:content+filtering+AND+abs:LLM"},

		// Code generation — developer tools market
		{"code-generation", "cat:cs.SE+AND+abs:code+generation+AND+abs:LLM"},
		{"code-repair", "abs:automated+program+repair+AND+abs:language+model"},
		{"code-review-ai", "abs:code+review+AND+abs:LLM"},

		// Structured output & parsing — API infrastructure
		{"structured-output", "abs:structured+output+AND+abs:language+model"},
		{"json-generation", "abs:JSON+AND+abs:language+model+AND+abs:generation"},
		{"function-calling", "abs:function+calling+AND+abs:LLM"},

		// Cost optimization — every company cares
		{"llm-routing", "abs:LLM+AND+abs:routing+AND+abs:cost"},
		{"model-cascading", "abs:model+cascade+AND+abs:language+model"},
		{"token-optimization", "abs:token+AND+abs:optimization+AND+abs:language+model"},

		// Context & memory — scaling limits
		{"long-context", "abs:long+context+AND+abs:language+model"},
		{"context-compression", "abs:context+compression+AND+abs:LLM"},
		{"memory-augmented", "abs:memory+AND+abs:language+model+AND+abs:augmented"},

		// Multimodal — next frontier
		{"vision-language", "abs:vision+language+model+AND+abs:tool"},
		{"multimodal-agent", "abs:multimodal+AND+abs:agent+AND+abs:LLM"},
	}
}

func (a *ArxivScraper) Scrape(ctx context.Context) ([]Item, error) {
	base := a.BaseURL
	if base == "" {
		base = "https://export.arxiv.org/api"
	}

	var allItems []Item
	for _, q := range a.queries() {
		searchURL := fmt.Sprintf("%s/query?search_query=%s&sortBy=submittedDate&sortOrder=descending&max_results=15",
			base, url.QueryEscape(q.query))

		items, err := a.fetch(ctx, searchURL, q.label)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)

		// Be polite to arXiv API — 1s between requests.
		select {
		case <-ctx.Done():
			return allItems, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return allItems, nil
}

func (a *ArxivScraper) fetch(ctx context.Context, fetchURL, label string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
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
			Title      string `xml:"title"`
			Summary    string `xml:"summary"`
			ID         string `xml:"id"`
			Published  string `xml:"published"`
			Categories []struct {
				Term string `xml:"term,attr"`
			} `xml:"category"`
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

		// Tag body with the query category for the analyzer.
		body := fmt.Sprintf("[arxiv:%s] %s", label, truncate(e.Summary, 2000))

		items = append(items, Item{
			Source: "arxiv",
			URL:    e.ID,
			Title:  e.Title,
			Body:   body,
			Author: author,
			Score:  0,
		})
	}
	return items, nil
}

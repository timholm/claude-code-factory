// Package llmrouter provides an HTTP client for the llm-router proxy.
// Used by the factory to route analyze requests through the router
// (gets semantic caching, model routing, dedup for free) and to
// report build feedback for threshold calibration.
package llmrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client communicates with an llm-router instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new llm-router client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// ChatMessage is a single message in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is an OpenAI-compatible chat completion request.
type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
}

// ChatResponse is the response from /v1/chat/completions.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete sends a chat completion request through llm-router.
// The router handles model selection, caching, and dedup.
func (c *Client) Complete(systemPrompt, userPrompt string) (string, error) {
	req := ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("llm-router: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm-router %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		// The router may return raw text if the upstream does.
		// Fall back to treating the whole body as the response.
		return string(respBody), nil
	}

	if len(chatResp.Choices) == 0 {
		return string(respBody), nil
	}
	return chatResp.Choices[0].Message.Content, nil
}

// FeedbackRequest reports build quality to the router.
type FeedbackRequest struct {
	Score   float64 `json:"score"`   // classifier score that was used
	Tier    int     `json:"tier"`    // which tier was used
	Quality float64 `json:"quality"` // 0-1 quality (1=tests pass, 0=build failed)
}

// SendFeedback reports build quality to llm-router for threshold calibration.
func (c *Client) SendFeedback(fb FeedbackRequest) error {
	body, err := json.Marshal(fb)
	if err != nil {
		return fmt.Errorf("marshal feedback: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/v1/feedback",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("send feedback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feedback: status %d", resp.StatusCode)
	}
	return nil
}

// ClassifyRequest asks llm-router to classify a prompt and return the recommended tier.
type ClassifyResponse struct {
	Score int    `json:"score"`
	Tier  int    `json:"tier"`
	Model string `json:"model"`
}

// Classify sends a prompt to llm-router and reads the routing headers
// to determine which model/tier would be used.
func (c *Client) Classify(prompt string) (*ClassifyResponse, error) {
	req := ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// Use HEAD-like approach: send to the router but we only care about headers.
	// Actually, we need to send a real request. Instead, we'll use the stats.
	// For now, just read the X-LLM-Router-* headers from a real request.
	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain

	score := 0
	tier := 2
	model := ""
	fmt.Sscanf(resp.Header.Get("X-LLM-Router-Score"), "%d", &score)
	fmt.Sscanf(resp.Header.Get("X-LLM-Router-Tier"), "%d", &tier)
	model = resp.Header.Get("X-LLM-Router-Model")

	return &ClassifyResponse{Score: score, Tier: tier, Model: model}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Package analyze renders an analysis prompt, calls Claude headlessly,
// parses the JSON specs from the response, and enqueues them for building.
package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

// Spec describes a single software project to build.
type Spec struct {
	Name           string   `json:"name"`
	Problem        string   `json:"problem"`
	SourceURL      string   `json:"source_url"`
	Solution       string   `json:"solution"`
	Language       string   `json:"language"`
	Files          []string `json:"files"`
	EstimatedLines int      `json:"estimated_lines"`
}

// templateData is the data passed to the prompt template.
type templateData struct {
	Items []registry.RawItem
}

// jsonArrayRE matches the first JSON array in a string (handles nested brackets).
var jsonArrayRE = regexp.MustCompile(`(?s)\[.*\]`)

// jsonCodeBlockRE extracts a JSON array from a ```json ... ``` fenced block.
var jsonCodeBlockRE = regexp.MustCompile("(?s)```json\\s*\\n(\\[.*?\\])\\s*\\n```")

// ParseSpecs extracts a []Spec from Claude's raw response text.
// It handles three forms:
//  1. A bare JSON array
//  2. A JSON array wrapped in ```json ... ``` fences
//  3. A JSON array embedded in surrounding prose
func ParseSpecs(raw string) ([]Spec, error) {
	candidate := strings.TrimSpace(raw)

	// Try ```json fenced block first — most structured response.
	if m := jsonCodeBlockRE.FindStringSubmatch(candidate); len(m) == 2 {
		candidate = strings.TrimSpace(m[1])
	} else if !strings.HasPrefix(candidate, "[") {
		// Fall back: find first '[' ... last ']' in the text.
		if m := jsonArrayRE.FindString(candidate); m != "" {
			candidate = m
		}
	}

	var specs []Spec
	if err := json.Unmarshal([]byte(candidate), &specs); err != nil {
		return nil, fmt.Errorf("analyze.ParseSpecs: %w", err)
	}
	return specs, nil
}

// renderPrompt renders the analyze.md.tmpl template with the given items.
func renderPrompt(promptsDir string, items []registry.RawItem) (string, error) {
	tmplPath := promptsDir + "/analyze.md.tmpl"
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("analyze.renderPrompt: read template %q: %w", tmplPath, err)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("analyze").Funcs(funcMap).Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("analyze.renderPrompt: parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData{Items: items}); err != nil {
		return "", fmt.Errorf("analyze.renderPrompt: execute template: %w", err)
	}
	return buf.String(), nil
}

// Run is the full analyze pipeline:
//  1. Fetch unfed items from the registry.
//  2. Render the prompt template.
//  3. Call Claude via the headless CLI.
//  4. Parse specs from Claude's output.
//  5. Enqueue each spec for building.
//  6. Mark items as fed.
func Run(ctx context.Context, reg *registry.Registry, claudeBinary, promptsDir string) error {
	items, err := reg.GetUnfedItems()
	if err != nil {
		return fmt.Errorf("analyze.Run: get unfed items: %w", err)
	}
	if len(items) == 0 {
		fmt.Println("analyze: no unfed items to process")
		return nil
	}
	fmt.Printf("analyze: processing %d unfed items\n", len(items))

	prompt, err := renderPrompt(promptsDir, items)
	if err != nil {
		return fmt.Errorf("analyze.Run: render prompt: %w", err)
	}

	raw, err := callClaude(ctx, claudeBinary, prompt)
	if err != nil {
		return fmt.Errorf("analyze.Run: call claude: %w", err)
	}

	specs, err := ParseSpecs(raw)
	if err != nil {
		return fmt.Errorf("analyze.Run: parse specs: %w", err)
	}
	fmt.Printf("analyze: parsed %d specs from Claude\n", len(specs))

	for _, s := range specs {
		filesJSON, err := json.Marshal(s.Files)
		if err != nil {
			return fmt.Errorf("analyze.Run: marshal files for %q: %w", s.Name, err)
		}
		bs := registry.BuildSpec{
			Name:           s.Name,
			Problem:        s.Problem,
			SourceURL:      s.SourceURL,
			Solution:       s.Solution,
			Language:       s.Language,
			Files:          string(filesJSON),
			EstimatedLines: s.EstimatedLines,
		}
		if err := reg.EnqueueSpec(bs); err != nil {
			return fmt.Errorf("analyze.Run: enqueue spec %q: %w", s.Name, err)
		}
	}

	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	if err := reg.MarkItemsFed(ids); err != nil {
		return fmt.Errorf("analyze.Run: mark items fed: %w", err)
	}

	fmt.Printf("analyze: enqueued %d specs, marked %d items as fed\n", len(specs), len(ids))
	return nil
}

// callClaude invokes the Claude CLI headlessly and returns its stdout.
func callClaude(ctx context.Context, claudeBinary, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, claudeBinary,
		"-p", prompt,
		"--output-format", "text",
		"--max-turns", "5",
	)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("callClaude: exec %q: %w", claudeBinary, err)
	}
	return string(out), nil
}

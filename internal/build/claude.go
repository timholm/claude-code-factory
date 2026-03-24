package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ClaudeResult holds the output of a Claude headless invocation.
type ClaudeResult struct {
	Output    string
	ExitCode  int
	RateLimit bool
}

// ClaudeOpts configures a Claude Code invocation.
type ClaudeOpts struct {
	Prompt         string
	MaxTurns       int
	Model          string   // optional: "sonnet", "haiku", "opus" (only used when no router)
	SystemPrompt   string   // optional: separate system instructions
	AllowedTools   []string // optional: restrict available tools
	RouterURL      string   // optional: route all API calls through llm-router
}

// InvokeClaude runs Claude headlessly with the given options in workDir.
// It sets CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 and uses --output-format text.
//
// When RouterURL is set, ALL of Claude Code's API calls flow through
// llm-router by setting ANTHROPIC_BASE_URL. The router then handles:
//   - Model selection (classifier picks cheapest capable model)
//   - Semantic caching (similar prompts return cached responses)
//   - Request deduplication (parallel builds don't double-pay)
//   - Health tracking (marks degraded/down backends)
//   - Cost tracking (per-request cost attribution)
//   - Feedback-driven threshold calibration
//
// When RouterURL is empty, uses --model flag for direct model selection.
func InvokeClaude(ctx context.Context, binary, workDir, prompt string, maxTurns int) (*ClaudeResult, error) {
	return InvokeClaudeWithOpts(ctx, binary, workDir, ClaudeOpts{
		Prompt:   prompt,
		MaxTurns: maxTurns,
	})
}

// InvokeClaudeWithOpts runs Claude headlessly with full options control.
func InvokeClaudeWithOpts(ctx context.Context, binary, workDir string, opts ClaudeOpts) (*ClaudeResult, error) {
	args := []string{
		"-p", opts.Prompt,
		"--permission-mode", "acceptEdits",
		"--max-turns", fmt.Sprintf("%d", opts.MaxTurns),
		"--output-format", "text",
	}

	// When routing through llm-router, DON'T pass --model — let the router classify
	// and pick the cheapest capable model. When NOT routing, use --model for direct control.
	if opts.RouterURL == "" && opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// System prompt: separate context from instructions.
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	// Tool restrictions: don't let SEO phase run code, etc.
	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir

	// Build environment — strip secrets so Claude can't embed them in generated code.
	var env []string
	for _, e := range os.Environ() {
		// Don't pass tokens/keys to Claude Code
		if strings.HasPrefix(e, "GITHUB_TOKEN=") ||
			strings.HasPrefix(e, "ANTHROPIC_API_KEY=") ||
			strings.HasPrefix(e, "OPENAI_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")

	// Route all Claude Code API calls through llm-router.
	if opts.RouterURL != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+strings.TrimRight(opts.RouterURL, "/"))
	}

	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("InvokeClaude: run: %w", runErr)
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	rateLimited := isRateLimited(output)

	return &ClaudeResult{
		Output:    output,
		ExitCode:  exitCode,
		RateLimit: rateLimited,
	}, nil
}

// isRateLimited returns true if output contains known rate-limit indicators.
func isRateLimited(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "usage limit") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "capacity")
}

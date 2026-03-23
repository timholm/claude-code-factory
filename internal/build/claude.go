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
	Model          string   // optional: "sonnet", "haiku", "opus"
	SystemPrompt   string   // optional: separate system instructions
	AllowedTools   []string // optional: restrict available tools
}

// InvokeClaude runs Claude headlessly with the given options in workDir.
// It sets CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 and uses --output-format text.
// Rate-limit detection checks for the strings "usage limit", "rate limit", or "capacity".
func InvokeClaude(ctx context.Context, binary, workDir, prompt string, maxTurns int) (*ClaudeResult, error) {
	return InvokeClaudeWithOpts(ctx, binary, workDir, ClaudeOpts{
		Prompt:   prompt,
		MaxTurns: maxTurns,
	})
}

// InvokeClaudeWithOpts runs Claude headlessly with full options control.
// This is the optimized path — allows model selection, tool restrictions,
// and system prompts for maximum efficiency per phase.
func InvokeClaudeWithOpts(ctx context.Context, binary, workDir string, opts ClaudeOpts) (*ClaudeResult, error) {
	args := []string{
		"-p", opts.Prompt,
		"--permission-mode", "acceptEdits",
		"--max-turns", fmt.Sprintf("%d", opts.MaxTurns),
		"--output-format", "text",
	}

	// Model selection: use cheaper models for simple tasks.
	if opts.Model != "" {
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
	cmd.Env = append(os.Environ(), "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")

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

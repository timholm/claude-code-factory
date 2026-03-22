package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeResult holds the output of a Claude headless invocation.
type ClaudeResult struct {
	Output    string
	ExitCode  int
	RateLimit bool
}

// InvokeClaude runs Claude headlessly against the SPEC.md found in workDir.
// It sets CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 and uses --output-format text.
// Rate-limit detection checks for the strings "usage limit", "rate limit", or "capacity".
func InvokeClaude(ctx context.Context, binary, workDir string) (*ClaudeResult, error) {
	specPath := filepath.Join(workDir, "SPEC.md")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("InvokeClaude: read SPEC.md: %w", err)
	}
	specContent := string(specBytes)

	args := []string{
		"-p", specContent,
		"--permission-mode", "acceptEdits",
		"--max-turns", "15",
		"--output-format", "text",
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

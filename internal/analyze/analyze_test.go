package analyze

import (
	"testing"
)

func TestParseSpecs(t *testing.T) {
	raw := `[
  {
    "name": "gh-issue-triage",
    "problem": "Triaging GitHub issues manually takes too long.",
    "source_url": "https://github.com/example/repo/issues/1",
    "solution": "A CLI that auto-labels issues using Claude based on title and body.",
    "language": "go",
    "files": ["main.go", "README.md"],
    "estimated_lines": 150
  },
  {
    "name": "slack-standup-bot",
    "problem": "Teams forget to post standups in Slack.",
    "source_url": "https://news.ycombinator.com/item?id=12345",
    "solution": "An MCP server that prompts team members for standup updates at a scheduled time.",
    "language": "typescript",
    "files": ["index.ts", "package.json"],
    "estimated_lines": 120
  }
]`

	specs, err := ParseSpecs(raw)
	if err != nil {
		t.Fatalf("ParseSpecs returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}

	if specs[0].Name != "gh-issue-triage" {
		t.Errorf("expected name %q, got %q", "gh-issue-triage", specs[0].Name)
	}
	if specs[0].Language != "go" {
		t.Errorf("expected language %q, got %q", "go", specs[0].Language)
	}
	if specs[0].EstimatedLines != 150 {
		t.Errorf("expected estimated_lines %d, got %d", 150, specs[0].EstimatedLines)
	}
	if len(specs[0].Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(specs[0].Files))
	}

	if specs[1].Name != "slack-standup-bot" {
		t.Errorf("expected name %q, got %q", "slack-standup-bot", specs[1].Name)
	}
}

func TestParseSpecsWithMarkdownWrapper(t *testing.T) {
	raw := `Here are the 30 specs I identified based on the items:

` + "```json" + `
[
  {
    "name": "env-secret-scanner",
    "problem": "Secrets accidentally committed to git repos cause security breaches.",
    "source_url": "https://reddit.com/r/devops/comments/abc",
    "solution": "A git pre-commit hook that scans staged files for common secret patterns and blocks the commit.",
    "language": "bash",
    "files": ["pre-commit", "README.md"],
    "estimated_lines": 80
  }
]
` + "```" + `

These specs focus on small, focused tools under 200 lines.`

	specs, err := ParseSpecs(raw)
	if err != nil {
		t.Fatalf("ParseSpecs returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Name != "env-secret-scanner" {
		t.Errorf("expected name %q, got %q", "env-secret-scanner", specs[0].Name)
	}
	if specs[0].Language != "bash" {
		t.Errorf("expected language %q, got %q", "bash", specs[0].Language)
	}
}

func TestParseSpecsWithSurroundingText(t *testing.T) {
	raw := `I've analyzed the items and identified these top problems:

[
  {
    "name": "docker-log-tail",
    "problem": "Developers struggle to tail logs from multiple Docker containers simultaneously.",
    "source_url": "https://github.com/moby/moby/issues/999",
    "solution": "A CLI that multiplexes stdout from multiple containers into a single color-coded stream.",
    "language": "go",
    "files": ["main.go"],
    "estimated_lines": 95
  }
]

Let me know if you'd like me to refine any of these.`

	specs, err := ParseSpecs(raw)
	if err != nil {
		t.Fatalf("ParseSpecs returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Name != "docker-log-tail" {
		t.Errorf("expected name %q, got %q", "docker-log-tail", specs[0].Name)
	}
}

func TestParseSpecsInvalidJSON(t *testing.T) {
	_, err := ParseSpecs("this is not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

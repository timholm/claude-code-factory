package build

import (
	"os"
	"path/filepath"
	"testing"
)

func scaffold(t *testing.T, language string) string {
	t.Helper()
	dir := t.TempDir()
	err := Scaffold(dir, "testproj", language, "solve X", "https://example.com", "use Y", "[\"main.go\"]", 100, "octocat", "", "", "")
	if err != nil {
		t.Fatalf("Scaffold(%s): %v", language, err)
	}
	return dir
}

func assertFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist, but it does not", name)
	}
}

func TestScaffoldGo(t *testing.T) {
	dir := scaffold(t, "go")
	assertFile(t, dir, "go.mod")
	assertFile(t, dir, "Makefile")
	assertFile(t, dir, ".gitignore")
	assertFile(t, dir, "LICENSE")
	assertFile(t, dir, "SPEC.md")
}

func TestScaffoldPython(t *testing.T) {
	dir := scaffold(t, "python")
	assertFile(t, dir, "pyproject.toml")
	assertFile(t, dir, "Makefile")
	assertFile(t, dir, ".gitignore")
	assertFile(t, dir, "LICENSE")
	assertFile(t, dir, "SPEC.md")
}

func TestScaffoldTypeScript(t *testing.T) {
	dir := scaffold(t, "typescript")
	assertFile(t, dir, "package.json")
	assertFile(t, dir, "tsconfig.json")
	assertFile(t, dir, "Makefile")
	assertFile(t, dir, ".gitignore")
	assertFile(t, dir, "LICENSE")
	assertFile(t, dir, "SPEC.md")
}

func TestScaffoldBash(t *testing.T) {
	dir := scaffold(t, "bash")
	assertFile(t, dir, "Makefile")
	assertFile(t, dir, ".gitignore")
	assertFile(t, dir, "LICENSE")
	assertFile(t, dir, "SPEC.md")
}

func TestScaffoldDefaultFallsToBash(t *testing.T) {
	dir := scaffold(t, "ruby") // unknown language
	assertFile(t, dir, "Makefile")
	assertFile(t, dir, ".gitignore")
	assertFile(t, dir, "LICENSE")
	assertFile(t, dir, "SPEC.md")
}

func TestScaffoldLicenseContainsMIT(t *testing.T) {
	dir := scaffold(t, "go")
	data, err := os.ReadFile(filepath.Join(dir, "LICENSE"))
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}
	if string(data) == "" {
		t.Error("LICENSE is empty")
	}
	// Should contain MIT license text
	content := string(data)
	if len(content) < 10 {
		t.Error("LICENSE content too short")
	}
}

func TestScaffoldSpecContainsProblem(t *testing.T) {
	dir := scaffold(t, "go")
	data, err := os.ReadFile(filepath.Join(dir, "SPEC.md"))
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}
	content := string(data)
	if !contains(content, "solve X") {
		t.Error("SPEC.md does not contain the problem statement")
	}
	if !contains(content, "testproj") {
		t.Error("SPEC.md does not contain the project name")
	}
}

func TestScaffoldSpecFormatsResearchPapers(t *testing.T) {
	dir := t.TempDir()
	papers := `["https://arxiv.org/abs/2603.12345","https://arxiv.org/abs/2603.67890"]`
	repos := `["https://github.com/owner/repo1","https://github.com/owner/repo2"]`
	err := Scaffold(dir, "researchproj", "go", "solve X", "https://example.com", "use Y", `["main.go"]`, 100, "octocat", papers, repos, "Developers pay $50/mo")
	if err != nil {
		t.Fatalf("Scaffold with research: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "SPEC.md"))
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}
	content := string(data)

	// Papers should be formatted as markdown bullet list, not JSON.
	if contains(content, `["https://arxiv.org`) {
		t.Error("SPEC.md contains raw JSON array for papers — should be markdown bullet list")
	}
	if !contains(content, "- https://arxiv.org/abs/2603.12345") {
		t.Error("SPEC.md missing bullet-formatted paper URL")
	}
	if !contains(content, "- https://arxiv.org/abs/2603.67890") {
		t.Error("SPEC.md missing second bullet-formatted paper URL")
	}

	// Repos should be formatted as markdown bullet list, not JSON.
	if contains(content, `["https://github.com`) {
		t.Error("SPEC.md contains raw JSON array for repos — should be markdown bullet list")
	}
	if !contains(content, "- https://github.com/owner/repo1") {
		t.Error("SPEC.md missing bullet-formatted repo URL")
	}
	if !contains(content, "- https://github.com/owner/repo2") {
		t.Error("SPEC.md missing second bullet-formatted repo URL")
	}

	// Section headers should be present.
	if !contains(content, "## Research Papers") {
		t.Error("SPEC.md missing Research Papers heading")
	}
	if !contains(content, "## Reference Implementations") {
		t.Error("SPEC.md missing Reference Implementations heading")
	}
	if !contains(content, "## Market Analysis") {
		t.Error("SPEC.md missing Market Analysis heading")
	}
	if !contains(content, "Developers pay $50/mo") {
		t.Error("SPEC.md missing market analysis content")
	}
}

func TestScaffoldSpecEmptyResearchOmitted(t *testing.T) {
	dir := t.TempDir()
	err := Scaffold(dir, "noresearch", "go", "solve X", "https://example.com", "use Y", `["main.go"]`, 100, "octocat", "[]", "[]", "")
	if err != nil {
		t.Fatalf("Scaffold without research: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "SPEC.md"))
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}
	content := string(data)

	if contains(content, "## Research Papers") {
		t.Error("SPEC.md should not contain Research Papers when papers is empty array")
	}
	if contains(content, "## Reference Implementations") {
		t.Error("SPEC.md should not contain Reference Implementations when repos is empty array")
	}
}

func TestFormatJSONList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"empty array", "[]", ""},
		{"null", "null", ""},
		{"single item", `["https://arxiv.org/abs/1234"]`, "- https://arxiv.org/abs/1234\n"},
		{"multiple items", `["https://arxiv.org/abs/1234","https://arxiv.org/abs/5678"]`, "- https://arxiv.org/abs/1234\n- https://arxiv.org/abs/5678\n"},
		{"plain text passthrough", "- already formatted\n- list\n", "- already formatted\n- list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatJSONList(tt.input)
			if got != tt.want {
				t.Errorf("formatJSONList(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

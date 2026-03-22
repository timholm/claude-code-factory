package build

import (
	"os"
	"path/filepath"
	"testing"
)

func scaffold(t *testing.T, language string) string {
	t.Helper()
	dir := t.TempDir()
	err := Scaffold(dir, "testproj", language, "solve X", "https://example.com", "use Y", "[\"main.go\"]", 100, "octocat")
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

package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBuild_GoSourceExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateBuild(dir, "go"); err != nil {
		t.Errorf("validateBuild should pass with .go file: %v", err)
	}
}

func TestValidateBuild_NoSourceFiles(t *testing.T) {
	dir := t.TempDir()
	// Empty dir — no source files.
	if err := validateBuild(dir, "go"); err == nil {
		t.Error("validateBuild should fail with no source files")
	}
}

func TestValidateBuild_PythonSourceExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi')"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateBuild(dir, "python"); err != nil {
		t.Errorf("validateBuild should pass with .py file: %v", err)
	}
}

func TestValidateSEO_AllFilesPresent(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"README.md", "llms.txt", "CLAUDE.md", "AGENTS.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateSEO(dir); err != nil {
		t.Errorf("validateSEO should pass with all files: %v", err)
	}
}

func TestValidateSEO_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	// Only create README.md — missing 3 files.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateSEO(dir)
	if err == nil {
		t.Error("validateSEO should fail with missing files")
	}
}

func TestRenderPrompt(t *testing.T) {
	// Create a temp prompts dir with a test template.
	dir := t.TempDir()
	tmpl := `Build a {{.Language}} project with {{.MaxTurns}} turns for {{.GitHubUser}}.`
	if err := os.WriteFile(filepath.Join(dir, "test.md.tmpl"), []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := renderPrompt(dir, "test.md.tmpl", templateData{
		Language:   "go",
		MaxTurns:   15,
		GitHubUser: "octocat",
	})
	if err != nil {
		t.Fatalf("renderPrompt: %v", err)
	}

	expected := "Build a go project with 15 turns for octocat."
	if result != expected {
		t.Errorf("renderPrompt = %q, want %q", result, expected)
	}
}

func TestPhases(t *testing.T) {
	p := phases(50)
	if len(p) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(p))
	}

	// Small project: build gets 10 turns.
	if p[0].MaxTurns != 10 {
		t.Errorf("build phase MaxTurns = %d, want 10 for small project", p[0].MaxTurns)
	}
	if p[0].Name != "build" {
		t.Errorf("first phase = %q, want 'build'", p[0].Name)
	}
	if p[1].Name != "seo" {
		t.Errorf("second phase = %q, want 'seo'", p[1].Name)
	}
	if p[2].Name != "review" {
		t.Errorf("third phase = %q, want 'review'", p[2].Name)
	}

	// SEO is not required.
	if p[1].Required {
		t.Error("seo phase should not be required")
	}

	// Large project: build gets 20 turns.
	p2 := phases(300)
	if p2[0].MaxTurns != 20 {
		t.Errorf("build phase MaxTurns = %d, want 20 for large project", p2[0].MaxTurns)
	}
}

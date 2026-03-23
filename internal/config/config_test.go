package config_test

import (
	"os"
	"testing"

	"github.com/timholmquist/claude-code-factory/internal/config"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_USER", "test-user")
	t.Setenv("FACTORY_DATA_DIR", "/tmp/data")
	t.Setenv("FACTORY_GIT_DIR", "/tmp/git")
	t.Setenv("CLAUDE_BINARY", "/usr/local/bin/claude")
	t.Setenv("IDEA_ENGINE_POSTGRES_URL", "postgres://localhost:5432/ideas")
	t.Setenv("FACTORY_SPECS_DIR", "/srv/specs")

	cfg := config.Load()

	if cfg.GitHubToken != "test-token" {
		t.Errorf("GitHubToken: got %q, want %q", cfg.GitHubToken, "test-token")
	}
	if cfg.GitHubUser != "test-user" {
		t.Errorf("GitHubUser: got %q, want %q", cfg.GitHubUser, "test-user")
	}
	if cfg.DataDir != "/tmp/data" {
		t.Errorf("DataDir: got %q, want %q", cfg.DataDir, "/tmp/data")
	}
	if cfg.GitDir != "/tmp/git" {
		t.Errorf("GitDir: got %q, want %q", cfg.GitDir, "/tmp/git")
	}
	if cfg.ClaudeBinary != "/usr/local/bin/claude" {
		t.Errorf("ClaudeBinary: got %q, want %q", cfg.ClaudeBinary, "/usr/local/bin/claude")
	}
	if cfg.MirrorDelaySec != 30 {
		t.Errorf("MirrorDelaySec: got %d, want 30", cfg.MirrorDelaySec)
	}
	if cfg.IdeaEnginePostgresURL != "postgres://localhost:5432/ideas" {
		t.Errorf("IdeaEnginePostgresURL: got %q, want %q", cfg.IdeaEnginePostgresURL, "postgres://localhost:5432/ideas")
	}
	if cfg.SpecsDir != "/srv/specs" {
		t.Errorf("SpecsDir: got %q, want %q", cfg.SpecsDir, "/srv/specs")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_USER")
	os.Unsetenv("FACTORY_DATA_DIR")
	os.Unsetenv("FACTORY_GIT_DIR")
	os.Unsetenv("CLAUDE_BINARY")
	os.Unsetenv("IDEA_ENGINE_POSTGRES_URL")
	os.Unsetenv("FACTORY_SPECS_DIR")

	cfg := config.Load()

	if cfg.GitHubToken != "" {
		t.Errorf("GitHubToken: got %q, want empty string", cfg.GitHubToken)
	}
	if cfg.GitHubUser != "" {
		t.Errorf("GitHubUser: got %q, want empty string", cfg.GitHubUser)
	}
	if cfg.DataDir != "/srv/factory" {
		t.Errorf("DataDir: got %q, want %q", cfg.DataDir, "/srv/factory")
	}
	if cfg.GitDir != "/srv/git" {
		t.Errorf("GitDir: got %q, want %q", cfg.GitDir, "/srv/git")
	}
	if cfg.ClaudeBinary != "claude" {
		t.Errorf("ClaudeBinary: got %q, want %q", cfg.ClaudeBinary, "claude")
	}
	if cfg.MirrorDelaySec != 30 {
		t.Errorf("MirrorDelaySec: got %d, want 30", cfg.MirrorDelaySec)
	}
	if cfg.IdeaEnginePostgresURL != "" {
		t.Errorf("IdeaEnginePostgresURL: got %q, want empty string", cfg.IdeaEnginePostgresURL)
	}
	if cfg.SpecsDir != "" {
		t.Errorf("SpecsDir: got %q, want empty string", cfg.SpecsDir)
	}
}

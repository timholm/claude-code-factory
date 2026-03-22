package registry

import (
	"testing"
)

func TestInsertAndQueryRepo(t *testing.T) {
	r := testDB(t)

	repo := Repo{
		Name:      "hello-world",
		Language:  "go",
		Problem:   "say hello",
		SourceURL: "https://example.com/1",
	}

	if err := r.InsertRepo(repo); err != nil {
		t.Fatalf("InsertRepo: %v", err)
	}

	// Duplicate insert should be silently ignored.
	if err := r.InsertRepo(repo); err != nil {
		t.Fatalf("duplicate InsertRepo: %v", err)
	}

	repos, err := r.GetUnpushedRepos()
	if err != nil {
		t.Fatalf("GetUnpushedRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 unpushed repo, got %d", len(repos))
	}
	got := repos[0]
	if got.Name != repo.Name {
		t.Errorf("Name: got %q, want %q", got.Name, repo.Name)
	}
	if got.Language != repo.Language {
		t.Errorf("Language: got %q, want %q", got.Language, repo.Language)
	}
	if got.Problem != repo.Problem {
		t.Errorf("Problem: got %q, want %q", got.Problem, repo.Problem)
	}
	if got.SourceURL != repo.SourceURL {
		t.Errorf("SourceURL: got %q, want %q", got.SourceURL, repo.SourceURL)
	}
}

func TestMarkPushed(t *testing.T) {
	r := testDB(t)

	repo := Repo{
		Name:      "my-tool",
		Language:  "go",
		Problem:   "does stuff",
		SourceURL: "https://example.com/2",
	}

	if err := r.InsertRepo(repo); err != nil {
		t.Fatalf("InsertRepo: %v", err)
	}

	if err := r.MarkPushed(repo.Name); err != nil {
		t.Fatalf("MarkPushed: %v", err)
	}

	repos, err := r.GetUnpushedRepos()
	if err != nil {
		t.Fatalf("GetUnpushedRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 unpushed repos after MarkPushed, got %d", len(repos))
	}
}

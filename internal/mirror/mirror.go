package mirror

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

// Run queries the registry for unpushed repos and pushes each one to GitHub.
// It creates the remote repo (best-effort, ignoring errors if it already exists),
// pushes via git --mirror, marks the repo as pushed in the registry, and sleeps
// for delay between pushes to avoid rate-limiting.
func Run(ctx context.Context, reg *registry.Registry, gitDir, ghUser, ghToken string, delay time.Duration) error {
	repos, err := reg.GetUnpushedRepos()
	if err != nil {
		return fmt.Errorf("mirror: query unpushed repos: %w", err)
	}

	if len(repos) == 0 {
		log.Println("mirror: no unpushed repos, nothing to do")
		return nil
	}

	log.Printf("mirror: %d repo(s) to push", len(repos))

	for i, repo := range repos {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("mirror: context cancelled: %w", err)
		}

		if err := mirrorRepo(ctx, repo, gitDir, ghUser, ghToken); err != nil {
			log.Printf("mirror: ERROR pushing %s: %v", repo.Name, err)
			// Continue to next repo on failure.
			continue
		}

		if err := reg.MarkPushed(repo.Name); err != nil {
			log.Printf("mirror: ERROR marking %s as pushed: %v", repo.Name, err)
		}

		// Sleep between pushes, but not after the last one.
		if i < len(repos)-1 {
			stagger(ctx, delay)
		}
	}

	return nil
}

// mirrorRepo creates the GitHub repo (best-effort) and pushes the bare git repo
// via --mirror.
func mirrorRepo(ctx context.Context, repo registry.Repo, gitDir, ghUser, ghToken string) error {
	repoSlug := ghUser + "/" + repo.Name

	// Create the GitHub repo — ignore errors since it may already exist.
	createArgs := []string{
		"repo", "create", repoSlug,
		"--public",
		"--description", repo.Problem,
		"--confirm",
	}
	createCmd := exec.CommandContext(ctx, "gh", createArgs...)
	if out, err := createCmd.CombinedOutput(); err != nil {
		log.Printf("mirror: gh repo create %s (best-effort, may already exist): %v — %s", repoSlug, err, out)
	} else {
		log.Printf("mirror: created GitHub repo %s", repoSlug)
	}

	// Push via --mirror using the HTTPS URL with embedded credentials.
	remoteURL := fmt.Sprintf("https://%s:%s@github.com/%s.git", ghUser, ghToken, repoSlug)
	bareDir := filepath.Join(gitDir, repo.Name+".git")

	pushCmd := exec.CommandContext(ctx, "git", "push", "--mirror", remoteURL)
	pushCmd.Dir = bareDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push --mirror for %s: %w — %s", repo.Name, err, out)
	}

	log.Printf("mirror: pushed %s to GitHub", repo.Name)
	return nil
}

// stagger sleeps for d, respecting context cancellation.
func stagger(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

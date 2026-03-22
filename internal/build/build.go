package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

const (
	buildBaseDir   = "/tmp/factory-build"
	rateLimitSleep = 30 * time.Minute
	maxAttempts    = 3
)

// Run is the main build loop. It dequeues items from the registry, scaffolds
// boilerplate, invokes Claude to implement the project, validates the output,
// commits it to a bare git repo, and marks the item as shipped.
// It returns when the queue is empty.
func Run(ctx context.Context, reg *registry.Registry, claudeBinary, gitDir, ghUser string) error {
	for {
		spec, err := reg.DequeueNext()
		if err != nil {
			return fmt.Errorf("build.Run: dequeue: %w", err)
		}
		if spec == nil {
			log.Println("build: queue empty, done")
			return nil
		}

		log.Printf("build: processing %s (attempt %d)", spec.Name, spec.Attempts)

		if err := processSpec(ctx, reg, spec, claudeBinary, gitDir, ghUser); err != nil {
			log.Printf("build: %s failed: %v", spec.Name, err)
		}
	}
}

// processSpec handles a single build spec: scaffold, invoke Claude, validate, git, register.
func processSpec(ctx context.Context, reg *registry.Registry, spec *registry.BuildSpec, claudeBinary, gitDir, ghUser string) error {
	workDir := filepath.Join(buildBaseDir, spec.Name)

	// Clean up workspace on exit.
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			log.Printf("build: cleanup %s: %v", workDir, err)
		}
	}()

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("mkdir workspace: %w", err))
	}

	// Scaffold boilerplate.
	if err := Scaffold(workDir, spec.Name, spec.Language, spec.Problem, spec.SourceURL, spec.Solution, spec.Files, spec.EstimatedLines, ghUser); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("scaffold: %w", err))
	}

	// Invoke Claude — with retry on rate limit.
	result, err := invokeWithRateLimitRetry(ctx, claudeBinary, workDir)
	if err != nil {
		return handleFailure(reg, spec, fmt.Errorf("claude: %w", err))
	}
	if result.ExitCode != 0 {
		return handleFailure(reg, spec, fmt.Errorf("claude exited %d: %s", result.ExitCode, truncate(result.Output, 500)))
	}

	// Validate: README.md must exist.
	if _, err := os.Stat(filepath.Join(workDir, "README.md")); os.IsNotExist(err) {
		return handleFailure(reg, spec, fmt.Errorf("validation: README.md not found"))
	}

	// Create bare git repo.
	bareRepo := filepath.Join(gitDir, spec.Name+".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("mkdir gitdir: %w", err))
	}
	if err := gitInitBare(bareRepo); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("git init bare: %w", err))
	}

	// Commit and tag in workspace, then push to bare repo.
	commitMsg := fmt.Sprintf("v0.1.0: %s", spec.Problem)
	if err := gitCommitAndPush(workDir, bareRepo, commitMsg); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("git commit/push: %w", err))
	}

	// Record in registry.
	if err := reg.InsertRepo(registry.Repo{
		Name:      spec.Name,
		Language:  spec.Language,
		Problem:   spec.Problem,
		SourceURL: spec.SourceURL,
	}); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("insert repo: %w", err))
	}

	if err := reg.MarkShipped(spec.ID); err != nil {
		return fmt.Errorf("mark shipped: %w", err)
	}

	log.Printf("build: shipped %s", spec.Name)
	return nil
}

// invokeWithRateLimitRetry calls InvokeClaude, sleeping and retrying if rate-limited.
func invokeWithRateLimitRetry(ctx context.Context, binary, workDir string) (*ClaudeResult, error) {
	for {
		result, err := InvokeClaude(ctx, binary, workDir)
		if err != nil {
			return nil, err
		}
		if !result.RateLimit {
			return result, nil
		}
		log.Printf("build: rate limited, sleeping %s", rateLimitSleep)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(rateLimitSleep):
		}
	}
}

// handleFailure decides whether to requeue or mark failed based on attempt count.
func handleFailure(reg *registry.Registry, spec *registry.BuildSpec, err error) error {
	log.Printf("build: failure for %s (attempts=%d): %v", spec.Name, spec.Attempts, err)
	if spec.Attempts >= maxAttempts {
		if mfErr := reg.MarkFailed(spec.ID, err.Error()); mfErr != nil {
			return fmt.Errorf("mark failed: %w (original: %v)", mfErr, err)
		}
		return nil
	}
	if rqErr := reg.RequeueForRetry(spec.ID); rqErr != nil {
		return fmt.Errorf("requeue: %w (original: %v)", rqErr, err)
	}
	return nil
}

// gitInitBare runs git init --bare in the given path.
func gitInitBare(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "init", "--bare", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init --bare: %w: %s", err, out)
	}
	return nil
}

// gitCommitAndPush initialises a git repo in workDir, commits all files with
// the given message, tags v0.1.0, and pushes to bareRepo.
func gitCommitAndPush(workDir, bareRepo, commitMsg string) error {
	gitEnv := []string{
		"GIT_AUTHOR_NAME=Factory",
		"GIT_AUTHOR_EMAIL=factory@localhost",
		"GIT_COMMITTER_NAME=Factory",
		"GIT_COMMITTER_EMAIL=factory@localhost",
	}

	type step struct {
		args []string
		env  []string
	}

	steps := []step{
		{args: []string{"init"}},
		{args: []string{"add", "-A"}},
		{args: []string{"commit", "-m", commitMsg}, env: gitEnv},
		{args: []string{"tag", "v0.1.0"}},
		{args: []string{"push", bareRepo, "HEAD:refs/heads/main", "--tags"}},
	}

	for _, s := range steps {
		cmd := exec.Command("git", s.args...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), s.env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %v: %w: %s", s.args, err, out)
		}
	}
	return nil
}

// truncate shortens a string to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

const (
	buildBaseDir   = "/tmp/factory-build"
	rateLimitSleep = 30 * time.Minute
	maxAttempts    = 1 // fail fast, move on, come back later
)

// Phase defines a single agent phase in the build pipeline.
type Phase struct {
	Name         string   // human-readable name for logging
	Status       string   // build_queue status while this phase is active
	TemplateName string   // filename in prompts dir (e.g. "build.md.tmpl")
	MaxTurns     int      // max turns for Claude
	Required     bool     // if true, failure stops the pipeline
	Model        string   // Claude model: "sonnet", "haiku", "opus" (empty = default)
	AllowedTools []string // restrict tools per phase (empty = all tools)
}

// phases returns the ordered pipeline phases for a given estimated line count.
func phases(estimatedLines int) []Phase {
	buildTurns := 15
	if estimatedLines <= 100 {
		buildTurns = 10
	}
	if estimatedLines > 200 {
		buildTurns = 20
	}

	return []Phase{
		{
			Name:         "build",
			Status:       "building",
			TemplateName: "build.md.tmpl",
			MaxTurns:     buildTurns,
			Required:     true,
			Model:        "sonnet", // Best balance of speed + quality for implementation
		},
		{
			Name:         "seo",
			Status:       "seo",
			TemplateName: "seo.md.tmpl",
			MaxTurns:     15,
			Required:     false,
			Model:        "sonnet", // Needs enough capability to read code + write good docs
			AllowedTools: []string{"Read", "Write", "Edit", "Glob", "Grep"}, // No Bash — SEO doesn't need to run code
		},
		{
			Name:         "review",
			Status:       "reviewing",
			TemplateName: "review.md.tmpl",
			MaxTurns:     10,
			Required:     true,
			Model:        "sonnet", // Needs to understand code + run tests
		},
	}
}

// templateData is passed to prompt templates.
type templateData struct {
	Language   string
	MaxTurns   int
	GitHubUser string
}

// BuildConfig holds runtime configuration for the build loop.
type BuildConfig struct {
	ClaudeBinary string
	GitDir       string
	GitHubUser   string
	RouterURL    string // llm-router URL for feedback + model selection (optional)
	Workers      int    // parallel build workers (default: 1)
}

// Run is the main build loop. It dequeues items from the registry, scaffolds
// boilerplate, runs the 3-agent pipeline (build → seo → review), validates,
// commits to a bare git repo, and marks as shipped.
//
// When Workers > 1, runs multiple builds in parallel with a semaphore.
// When RouterURL is set, reports build quality feedback to llm-router
// so the router's threshold auto-calibrates over time.
func Run(ctx context.Context, reg *registry.Registry, cfg BuildConfig) error {
	promptsDir := resolvePromptsDir()
	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}

	if workers == 1 {
		// Single-worker mode (original behavior).
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
			if err := processSpec(ctx, reg, spec, cfg.ClaudeBinary, cfg.GitDir, cfg.GitHubUser, promptsDir, cfg.RouterURL); err != nil {
				log.Printf("build: %s failed: %v", spec.Name, err)
			}
		}
	}

	// Multi-worker mode: run N builds in parallel.
	log.Printf("build: starting %d parallel workers", workers)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for {
		spec, err := reg.DequeueNext()
		if err != nil {
			return fmt.Errorf("build.Run: dequeue: %w", err)
		}
		if spec == nil {
			break
		}

		sem <- struct{}{} // acquire slot
		wg.Add(1)
		go func(s *registry.BuildSpec) {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			log.Printf("build: processing %s (attempt %d)", s.Name, s.Attempts)
			if err := processSpec(ctx, reg, s, cfg.ClaudeBinary, cfg.GitDir, cfg.GitHubUser, promptsDir, cfg.RouterURL); err != nil {
				log.Printf("build: %s failed: %v", s.Name, err)
			}
		}(spec)
	}

	wg.Wait()
	log.Println("build: queue empty, done")
	return nil
}

// processSpec handles a single build spec through the full 3-agent pipeline.
func processSpec(ctx context.Context, reg *registry.Registry, spec *registry.BuildSpec, claudeBinary, gitDir, ghUser, promptsDir, routerURL string) error {
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

	// Scaffold boilerplate (includes SPEC.md with references).
	if err := Scaffold(workDir, spec.Name, spec.Language, spec.Problem, spec.SourceURL, spec.Solution, spec.Files, spec.EstimatedLines, ghUser, spec.SourcePapers, spec.SourceRepos, spec.MarketAnalysis); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("scaffold: %w", err))
	}

	// Run the 3-agent pipeline.
	for _, phase := range phases(spec.EstimatedLines) {
		// Update status in registry.
		if err := reg.UpdateStatus(spec.ID, phase.Status); err != nil {
			return handleFailure(reg, spec, fmt.Errorf("update status to %s: %w", phase.Status, err))
		}

		log.Printf("build: %s — phase %s (max-turns %d)", spec.Name, phase.Name, phase.MaxTurns)

		// Render the prompt template.
		prompt, err := renderPrompt(promptsDir, phase.TemplateName, templateData{
			Language:   spec.Language,
			MaxTurns:   phase.MaxTurns,
			GitHubUser: ghUser,
		})
		if err != nil {
			return handleFailure(reg, spec, fmt.Errorf("render prompt %s: %w", phase.TemplateName, err))
		}

		// Invoke Claude with rate-limit retry and phase-specific optimizations.
		// Build phases use Claude Code's native auth (higher rate limits).
		// The router is used only for analyze + idea-engine (direct API calls).
		opts := ClaudeOpts{
			Prompt:       prompt,
			MaxTurns:     phase.MaxTurns,
			Model:        phase.Model,
			AllowedTools: phase.AllowedTools,
		}
		result, err := invokeWithRateLimitRetryOpts(ctx, claudeBinary, workDir, opts)
		if err != nil {
			if phase.Required {
				return handleFailure(reg, spec, fmt.Errorf("%s: claude: %w", phase.Name, err))
			}
			log.Printf("build: %s — optional phase %s failed: %v, continuing", spec.Name, phase.Name, err)
			continue
		}
		if result.ExitCode != 0 {
			if phase.Required {
				return handleFailure(reg, spec, fmt.Errorf("%s: claude exited %d: %s", phase.Name, result.ExitCode, truncate(result.Output, 500)))
			}
			log.Printf("build: %s — optional phase %s exited %d, continuing", spec.Name, phase.Name, result.ExitCode)
			continue
		}

		// Run phase-specific validation.
		if err := validatePhase(phase.Name, workDir, spec.Language); err != nil {
			if phase.Required {
				return handleFailure(reg, spec, fmt.Errorf("%s validation: %w", phase.Name, err))
			}
			log.Printf("build: %s — optional phase %s validation failed: %v, continuing", spec.Name, phase.Name, err)
		}
	}

	// Pre-test dependency resolution: fix the #1 failure cause.
	if err := resolveDeps(workDir, spec.Language); err != nil {
		log.Printf("build: %s — dep resolution warning: %v", spec.Name, err)
	}

	// Final gate: make test must pass.
	if err := runMakeTest(workDir); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("final make test: %w", err))
	}

	// Create bare git repo.
	bareRepo := filepath.Join(gitDir, spec.Name+".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("mkdir gitdir: %w", err))
	}
	if err := gitInitBare(bareRepo); err != nil {
		return handleFailure(reg, spec, fmt.Errorf("git init bare: %w", err))
	}

	// Scrub secrets before committing — Claude sometimes embeds real tokens.
	if err := scrubSecrets(workDir); err != nil {
		log.Printf("build: %s — secret scrub warning: %v", spec.Name, err)
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

	// Report success feedback to llm-router for threshold calibration.
	if routerURL != "" {
		sendFeedback(routerURL, spec.EstimatedLines, 1.0) // quality=1.0 for shipped builds
	}

	log.Printf("build: shipped %s", spec.Name)
	return nil
}

// renderPrompt loads and executes a Go template from promptsDir.
func renderPrompt(promptsDir, templateName string, data templateData) (string, error) {
	tmplPath := filepath.Join(promptsDir, templateName)
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", tmplPath, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", templateName, err)
	}
	return buf.String(), nil
}

// validatePhase runs phase-specific validation checks.
func validatePhase(phase, workDir, language string) error {
	switch phase {
	case "build":
		return validateBuild(workDir, language)
	case "seo":
		return validateSEO(workDir)
	case "review":
		// Review's own validation is just "make test passes" — handled by final gate.
		return nil
	}
	return nil
}

// validateBuild checks that the build phase produced real source files.
func validateBuild(workDir, language string) error {
	// Check for at least one source file beyond scaffolding.
	var extensions []string
	switch strings.ToLower(language) {
	case "go":
		extensions = []string{".go"}
	case "python":
		extensions = []string{".py"}
	case "typescript", "ts":
		extensions = []string{".ts", ".js"}
	case "bash":
		extensions = []string{".sh"}
	case "rust":
		extensions = []string{".rs"}
	default:
		return nil // can't validate unknown languages
	}

	found := false
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, ext := range extensions {
			if strings.HasSuffix(info.Name(), ext) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})

	if !found {
		return fmt.Errorf("no %s source files found in %s", language, workDir)
	}
	return nil
}

// validateSEO checks that the LLM-SEO files were created.
func validateSEO(workDir string) error {
	required := []string{"README.md", "llms.txt", "CLAUDE.md", "AGENTS.md"}
	var missing []string
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(workDir, f)); os.IsNotExist(err) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing SEO files: %s", strings.Join(missing, ", "))
	}
	return nil
}

// resolveDeps runs language-specific dependency resolution before the final test gate.
// This catches the most common failure: Claude adding imports without running go mod tidy.
func resolveDeps(workDir, language string) error {
	switch strings.ToLower(language) {
	case "go":
		// Step 1: Scan all .go files for imports and fetch missing deps.
		// This is the #1 cause of build failures — Claude adds imports but never runs go get.
		commonDeps := []string{
			"github.com/spf13/cobra@latest",
			"github.com/spf13/viper@latest",
			"gopkg.in/yaml.v3@latest",
			"github.com/google/uuid@latest",
			"modernc.org/sqlite@latest",
		}
		for _, dep := range commonDeps {
			// Check if any .go file imports this package
			depBase := dep[:strings.Index(dep, "@")]
			grepCmd := exec.Command("grep", "-r", depBase, "--include=*.go", workDir)
			if grepCmd.Run() == nil {
				// This dep is used — make sure it's in go.mod
				getCmd := exec.Command("go", "get", dep)
				getCmd.Dir = workDir
				getCmd.CombinedOutput() // best-effort, ignore errors
			}
		}
		// Step 2: go mod tidy to clean up
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go mod tidy: %w: %s", err, truncate(string(out), 300))
		}
	case "typescript", "ts":
		// Run npm install if node_modules doesn't exist.
		if _, err := os.Stat(filepath.Join(workDir, "node_modules")); os.IsNotExist(err) {
			cmd := exec.Command("npm", "install")
			cmd.Dir = workDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("npm install: %w: %s", err, truncate(string(out), 300))
			}
		}
	}
	return nil
}

// runMakeTest runs `make test` in workDir and returns an error if it fails.
func runMakeTest(workDir string) error {
	// Check Makefile exists first.
	if _, err := os.Stat(filepath.Join(workDir, "Makefile")); os.IsNotExist(err) {
		return nil // no Makefile, skip test
	}

	cmd := exec.Command("make", "test")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make test failed: %w\n%s", err, truncate(string(out), 500))
	}
	return nil
}

// invokeWithRateLimitRetry calls InvokeClaude, sleeping and retrying if rate-limited.
func invokeWithRateLimitRetry(ctx context.Context, binary, workDir, prompt string, maxTurns int) (*ClaudeResult, error) {
	return invokeWithRateLimitRetryOpts(ctx, binary, workDir, ClaudeOpts{Prompt: prompt, MaxTurns: maxTurns})
}

// invokeWithRateLimitRetryOpts calls InvokeClaudeWithOpts, sleeping and retrying if rate-limited.
func invokeWithRateLimitRetryOpts(ctx context.Context, binary, workDir string, opts ClaudeOpts) (*ClaudeResult, error) {
	for {
		result, err := InvokeClaudeWithOpts(ctx, binary, workDir, opts)
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

// sendFeedback reports build quality to llm-router for threshold calibration.
func sendFeedback(routerURL string, estimatedLines int, quality float64) {
	// Map estimated lines to a complexity score (0-1).
	score := float64(estimatedLines) / 300.0
	if score > 1.0 {
		score = 1.0
	}

	// Determine tier from project size.
	tier := 1
	if estimatedLines > 100 {
		tier = 2
	}
	if estimatedLines > 200 {
		tier = 3
	}

	fb := struct {
		Score   float64 `json:"score"`
		Tier    int     `json:"tier"`
		Quality float64 `json:"quality"`
	}{
		Score:   score,
		Tier:    tier,
		Quality: quality,
	}

	body, _ := json.Marshal(fb)
	resp, err := http.Post(
		strings.TrimRight(routerURL, "/")+"/v1/feedback",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		log.Printf("build: feedback send failed: %v", err)
		return
	}
	resp.Body.Close()
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

// resolvePromptsDir returns the path to the prompts directory. It prefers a
// local ./prompts directory (for development) and falls back to the production
// path /etc/factory/prompts.
func resolvePromptsDir() string {
	if _, err := os.Stat("prompts"); err == nil {
		return "prompts"
	}
	return "/etc/factory/prompts"
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

// scrubSecrets removes GitHub tokens, API keys, and other secrets from all files
// before committing. Claude sometimes embeds real tokens in test fixtures or examples.
func scrubSecrets(workDir string) error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
		regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`),
		regexp.MustCompile(`gho_[A-Za-z0-9]{36}`),
		regexp.MustCompile(`sk-ant-[A-Za-z0-9\-_]{40,}`),
		regexp.MustCompile(`sk-[A-Za-z0-9]{48}`),
	}

	return filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.Contains(path, "/.git/") || info.Size() > 1<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		changed := false
		for _, re := range patterns {
			if re.MatchString(content) {
				content = re.ReplaceAllString(content, "REDACTED_SECRET")
				changed = true
				log.Printf("build: scrubbed secret from %s", filepath.Base(path))
			}
		}
		if changed {
			return os.WriteFile(path, []byte(content), info.Mode())
		}
		return nil
	})
}

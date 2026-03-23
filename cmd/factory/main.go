package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/timholmquist/claude-code-factory/internal/analyze"
	"github.com/timholmquist/claude-code-factory/internal/build"
	"github.com/timholmquist/claude-code-factory/internal/config"
	"github.com/timholmquist/claude-code-factory/internal/gather"
	"github.com/timholmquist/claude-code-factory/internal/importer"
	"github.com/timholmquist/claude-code-factory/internal/mirror"
	"github.com/timholmquist/claude-code-factory/internal/registry"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "factory",
		Short: "claude-code-factory: autonomous software factory",
		Long: `claude-code-factory is an autonomous software factory.

Primary workflow (idea-engine integration):
  import         - import product specs from JSON file or directory
  import-from-db - import product specs from idea-engine's Postgres
  build          - build software projects from queued specs
  mirror         - push built projects to GitHub

Legacy commands (fallback, use import instead):
  gather         - scrape data from the web
  analyze        - call Claude to analyze data`,
	}

	rootCmd.AddCommand(gatherCmd())
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(importCmd())
	rootCmd.AddCommand(importFromDBCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(mirrorCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func gatherCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gather",
		Short: "[legacy] Scrape data from the web",
		Long:  "[legacy] gather scrapes data sources and stores them for analysis. Use 'import' or 'import-from-db' instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			scrapers := []gather.Scraper{
				&gather.ArxivScraper{},
			}

			count, err := gather.Run(context.Background(), reg, scrapers)
			if err != nil {
				return err
			}
			fmt.Printf("gathered %d items\n", count)
			return nil
		},
	}
}

func analyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "[legacy] Analyze gathered data using Claude",
		Long:  "[legacy] analyze calls Claude to process gathered data and produce structured output. Use 'import' or 'import-from-db' instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			promptsDir := resolvePromptsDir()

			return analyze.Run(context.Background(), reg, cfg.ClaudeBinary, promptsDir, cfg.RouterURL)
		},
	}
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

func buildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build software projects from analysis output",
		Long:  "build dequeues specs, scaffolds boilerplate, invokes Claude, and ships bare git repos.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			return build.Run(context.Background(), reg, build.BuildConfig{
				ClaudeBinary: cfg.ClaudeBinary,
				GitDir:       cfg.GitDir,
				GitHubUser:   cfg.GitHubUser,
				RouterURL:    cfg.RouterURL,
				Workers:      cfg.BuildWorkers,
			})
		},
	}
}

func importCmd() *cobra.Command {
	var specsFile string
	var specsDir string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import product specs from idea-engine JSON files",
		Long:  "import reads product specs from a JSON file or directory of JSON files and enqueues them for building.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			if specsFile == "" && specsDir == "" {
				// Fall back to config env var.
				specsDir = cfg.SpecsDir
			}

			if specsFile == "" && specsDir == "" {
				return fmt.Errorf("specify --specs or --dir (or set FACTORY_SPECS_DIR)")
			}

			var count int
			if specsFile != "" {
				count, err = importer.ImportFromFile(specsFile, reg)
			} else {
				count, err = importer.ImportFromDir(specsDir, reg)
			}
			if err != nil {
				return err
			}

			fmt.Printf("imported %d specs into build queue\n", count)
			return nil
		},
	}

	cmd.Flags().StringVar(&specsFile, "specs", "", "path to a JSON file containing one or more product specs")
	cmd.Flags().StringVar(&specsDir, "dir", "", "path to a directory of JSON spec files")

	return cmd
}

func importFromDBCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-from-db",
		Short: "Import product specs from idea-engine's Postgres database",
		Long:  "import-from-db reads synthesized product specs from idea-engine's candidates table and enqueues them for building.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			postgresURL := cfg.IdeaEnginePostgresURL
			if postgresURL == "" {
				return fmt.Errorf("IDEA_ENGINE_POSTGRES_URL is required")
			}

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			count, err := importer.ImportFromDB(postgresURL, reg)
			if err != nil {
				return err
			}

			fmt.Printf("imported %d specs from idea-engine into build queue\n", count)
			return nil
		},
	}
}

func mirrorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mirror",
		Short: "Push built projects to GitHub",
		Long:  "mirror pushes locally built projects to remote GitHub repositories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			delay := time.Duration(cfg.MirrorDelaySec) * time.Second
			return mirror.Run(context.Background(), reg, cfg.GitDir, cfg.GitHubUser, cfg.GitHubToken, delay)
		},
	}
}

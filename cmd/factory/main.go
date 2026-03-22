package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/timholmquist/claude-code-factory/internal/analyze"
	"github.com/timholmquist/claude-code-factory/internal/config"
	"github.com/timholmquist/claude-code-factory/internal/gather"
	"github.com/timholmquist/claude-code-factory/internal/registry"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "factory",
		Short: "claude-code-factory: autonomous software factory",
		Long: `claude-code-factory is an autonomous software factory with four modes:
  gather  - scrape data from the web
  analyze - call Claude to analyze data
  build   - build software projects
  mirror  - push projects to GitHub`,
	}

	rootCmd.AddCommand(gatherCmd())
	rootCmd.AddCommand(analyzeCmd())
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
		Short: "Scrape data from the web",
		Long:  "gather scrapes data sources and stores them for analysis.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			scrapers := []gather.Scraper{
				&gather.GitHubIssuesScraper{Token: cfg.GitHubToken},
				&gather.HNScraper{},
				&gather.RedditScraper{UserAgent: cfg.RedditAgent},
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
		Short: "Analyze gathered data using Claude",
		Long:  "analyze calls Claude to process gathered data and produce structured output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := registry.Open(filepath.Join(cfg.DataDir, "registry.db"))
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer db.Close()
			reg := &registry.Registry{DB: db}

			promptsDir := resolvePromptsDir()

			return analyze.Run(context.Background(), reg, cfg.ClaudeBinary, promptsDir)
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
		Long:  "build takes analyzed data and scaffolds or builds software projects.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("build: not implemented")
		},
	}
}

func mirrorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mirror",
		Short: "Push built projects to GitHub",
		Long:  "mirror pushes locally built projects to remote GitHub repositories.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("mirror: not implemented")
		},
	}
}

package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for the factory.
type Config struct {
	GitHubToken    string
	GitHubUser     string
	DataDir        string
	GitDir         string
	ClaudeBinary   string
	MirrorDelaySec int
	RouterURL      string // llm-router URL for analyze/routing (optional, e.g. "http://localhost:8080")
	BuildWorkers   int    // number of parallel build workers (default: 1)
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Load reads configuration from environment variables, applying defaults where needed.
func Load() Config {
	workers := 1
	if w := os.Getenv("BUILD_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			workers = n
		}
	}

	return Config{
		GitHubToken:    os.Getenv("GITHUB_TOKEN"),
		GitHubUser:     os.Getenv("GITHUB_USER"),
		DataDir:        getenv("FACTORY_DATA_DIR", "/srv/factory"),
		GitDir:         getenv("FACTORY_GIT_DIR", "/srv/git"),
		ClaudeBinary:   getenv("CLAUDE_BINARY", "claude"),
		MirrorDelaySec: 30,
		RouterURL:      os.Getenv("LLM_ROUTER_URL"), // e.g. "http://localhost:8080"
		BuildWorkers:   workers,
	}
}

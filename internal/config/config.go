package config

import "os"

// Config holds all runtime configuration for the factory.
type Config struct {
	GitHubToken    string
	GitHubUser     string
	RedditAgent    string
	DataDir        string
	GitDir         string
	ClaudeBinary   string
	MirrorDelaySec int
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Load reads configuration from environment variables, applying defaults where needed.
func Load() Config {
	return Config{
		GitHubToken:    os.Getenv("GITHUB_TOKEN"),
		GitHubUser:     os.Getenv("GITHUB_USER"),
		RedditAgent:    getenv("REDDIT_USER_AGENT", "factory/1.0"),
		DataDir:        getenv("FACTORY_DATA_DIR", "/srv/factory"),
		GitDir:         getenv("FACTORY_GIT_DIR", "/srv/git"),
		ClaudeBinary:   getenv("CLAUDE_BINARY", "claude"),
		MirrorDelaySec: 30,
	}
}

// Package config loads server bootstrap configuration from the environment.
package config

import "os"

// Config holds server-wide settings read at startup.
type Config struct {
	Port   string
	DBPath string
}

// Load reads configuration from environment variables, applying defaults
// for anything not set.
func Load() Config {
	return Config{
		Port:   getEnv("PORT", "8080"),
		DBPath: getEnv("DB_PATH", "./data/mullet.db"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

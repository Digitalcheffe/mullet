// Package config loads server bootstrap configuration from the environment.
package config

import (
	"os"
	"strings"
)

// Config holds server-wide settings read at startup.
type Config struct {
	Port         string
	DBPath       string
	CORSOrigins  []string
	StaticDir    string
	AuthDisabled bool
}

// Load reads configuration from environment variables, applying defaults
// for anything not set.
func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		DBPath:       getEnv("DB_PATH", "./data/mullet.db"),
		CORSOrigins:  getEnvList("CORS_ORIGINS"),
		StaticDir:    getEnv("STATIC_DIR", "./web/dist"),
		AuthDisabled: getEnvBool("AUTH_DISABLED"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvBool reports whether key is set to a truthy value ("1", "true",
// case-insensitive). Anything else, including unset, is false.
func getEnvBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true"
}

// getEnvList parses a comma-separated env var into a trimmed, non-empty
// list of values. An unset or empty var yields nil (CORS disabled).
func getEnvList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}

	var values []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			values = append(values, v)
		}
	}
	return values
}

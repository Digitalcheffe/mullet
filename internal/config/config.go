// Package config loads server bootstrap configuration from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
)

// SMTPEnvConfig is outgoing-mail configuration read from the environment,
// for seeding db.SMTPConfig on first boot (see db.SeedSMTPConfigFromEnv)
// -- a fresh Docker deployment can have working password-reset/
// notification email without an admin visiting Settings first. Host
// empty means "nothing to seed."
type SMTPEnvConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	TLSMode     string
}

// Config holds server-wide settings read at startup.
type Config struct {
	Port         string
	DBPath       string
	UploadsDir   string
	CORSOrigins  []string
	StaticDir    string
	AuthDisabled bool
	// LogPath seeds the log file destination setting on first boot (see
	// db.SeedLogFilePathFromEnv) -- empty means "nothing to seed," not
	// "stdout only" (that's the DB setting's own empty-string meaning,
	// once something has actually seeded or saved it).
	LogPath string
	SMTP    SMTPEnvConfig
}

// Load reads configuration from environment variables, applying defaults
// for anything not set.
func Load() Config {
	return Config{
		Port:   getEnv("PORT", "8080"),
		DBPath: getEnv("DB_PATH", "./data/mullet.db"),
		// Defaults alongside DB_PATH under the same ./data volume (see
		// docker-compose.yml) rather than a second Docker volume --
		// uploaded files need to survive a container restart/update the
		// same way the database does.
		UploadsDir:   getEnv("UPLOADS_DIR", "./data/uploads"),
		CORSOrigins:  getEnvList("CORS_ORIGINS"),
		StaticDir:    getEnv("STATIC_DIR", "./web/dist"),
		AuthDisabled: getEnvBool("AUTH_DISABLED"),
		// No Go-level default (unlike DB_PATH/UPLOADS_DIR above) --
		// leaving this unset must keep today's stdout-only default for
		// every existing deployment; only the Docker image itself
		// opts into a default file path under /data (see Dockerfile).
		LogPath: getEnv("LOG_PATH", ""),
		SMTP: SMTPEnvConfig{
			Host:        getEnv("SMTP_HOST", ""),
			Port:        getEnvInt("SMTP_PORT", 587),
			Username:    getEnv("SMTP_USERNAME", ""),
			Password:    getEnv("SMTP_PASSWORD", ""),
			FromAddress: getEnv("SMTP_FROM_ADDRESS", ""),
			TLSMode:     getEnv("SMTP_TLS_MODE", "starttls"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt parses key as an integer, falling back on unset or
// unparseable input rather than failing startup over a typo'd port
// number.
func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
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

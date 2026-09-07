package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// GetSetting returns the value stored for key in system_settings, and
// false if no row exists for it.
func GetSetting(sqldb *sql.DB, key string) (string, bool, error) {
	var value string
	err := sqldb.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetSetting upserts key to value in system_settings.
func SetSetting(sqldb *sql.DB, key, value string) error {
	_, err := sqldb.Exec(
		`INSERT INTO system_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("writing setting %q: %w", key, err)
	}
	return nil
}

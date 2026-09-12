package db

import "database/sql"

const logFilePathSettingKey = "log_file_path"

// GetLogFilePath returns the saved log file path, or "" (stdout only)
// if none has ever been saved.
func GetLogFilePath(sqldb *sql.DB) (string, error) {
	path, _, err := GetSetting(sqldb, logFilePathSettingKey)
	return path, err
}

// SetLogFilePath persists path as the log file destination. An empty
// path means stdout only.
func SetLogFilePath(sqldb *sql.DB, path string) error {
	return SetSetting(sqldb, logFilePathSettingKey, path)
}

// SeedLogFilePathFromEnv saves path as the log file destination only if
// no log file path has ever been explicitly saved -- same
// seed-once-then-admin-wins behavior as SeedSMTPConfigFromEnv, so a
// Docker deployment's default log-under-/data path doesn't fight an
// admin who later changes it (including back to "" for stdout only) via
// Settings. No-op if path is empty.
func SeedLogFilePathFromEnv(sqldb *sql.DB, path string) error {
	if path == "" {
		return nil
	}
	_, found, err := GetSetting(sqldb, logFilePathSettingKey)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return SetLogFilePath(sqldb, path)
}

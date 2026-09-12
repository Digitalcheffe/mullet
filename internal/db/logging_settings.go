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

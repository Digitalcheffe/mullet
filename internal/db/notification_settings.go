package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const notificationPreferencesSettingKey = "notification_preferences"

// NotificationPreferences controls which security-relevant events (issue
// #112) email every configured admin. All off by default -- a fresh
// install sends nothing until an admin opts in from Settings.
type NotificationPreferences struct {
	NewUser                bool `json:"new_user"`
	PasswordResetRequested bool `json:"password_reset_requested"`
	PasswordResetCompleted bool `json:"password_reset_completed"`
	ClientRegistered       bool `json:"client_registered"`
	ClientApproved         bool `json:"client_approved"`
}

// GetNotificationPreferences returns the saved preferences, or the zero
// value (everything off) if none have ever been saved.
func GetNotificationPreferences(sqldb *sql.DB) (NotificationPreferences, error) {
	raw, found, err := GetSetting(sqldb, notificationPreferencesSettingKey)
	if err != nil {
		return NotificationPreferences{}, err
	}
	if !found {
		return NotificationPreferences{}, nil
	}
	var prefs NotificationPreferences
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return NotificationPreferences{}, fmt.Errorf("decoding stored notification preferences: %w", err)
	}
	return prefs, nil
}

// SaveNotificationPreferences persists prefs, replacing whatever was
// saved before.
func SaveNotificationPreferences(sqldb *sql.DB, prefs NotificationPreferences) error {
	data, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("encoding notification preferences: %w", err)
	}
	return SetSetting(sqldb, notificationPreferencesSettingKey, string(data))
}

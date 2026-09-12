package db

import "testing"

func TestGetNotificationPreferencesDefaultsToAllOff(t *testing.T) {
	sqldb := newTestDB(t)

	prefs, err := GetNotificationPreferences(sqldb)
	if err != nil {
		t.Fatalf("GetNotificationPreferences: %v", err)
	}
	if prefs != (NotificationPreferences{}) {
		t.Errorf("GetNotificationPreferences (unsaved) = %+v, want all false", prefs)
	}
}

func TestNotificationPreferencesRoundTrip(t *testing.T) {
	sqldb := newTestDB(t)

	want := NotificationPreferences{
		NewUser:                true,
		PasswordResetRequested: true,
		ClientApproved:         true,
	}
	if err := SaveNotificationPreferences(sqldb, want); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	got, err := GetNotificationPreferences(sqldb)
	if err != nil {
		t.Fatalf("GetNotificationPreferences: %v", err)
	}
	if got != want {
		t.Errorf("GetNotificationPreferences = %+v, want %+v", got, want)
	}
}

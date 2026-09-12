package db

import "testing"

func TestGetLogFilePathDefaultsToEmpty(t *testing.T) {
	sqldb := newTestDB(t)
	path, err := GetLogFilePath(sqldb)
	if err != nil {
		t.Fatalf("GetLogFilePath: %v", err)
	}
	if path != "" {
		t.Errorf("GetLogFilePath (unsaved) = %q, want empty (stdout only)", path)
	}
}

func TestLogFilePathRoundTrip(t *testing.T) {
	sqldb := newTestDB(t)
	if err := SetLogFilePath(sqldb, "/var/log/mullet.log"); err != nil {
		t.Fatalf("SetLogFilePath: %v", err)
	}
	got, err := GetLogFilePath(sqldb)
	if err != nil {
		t.Fatalf("GetLogFilePath: %v", err)
	}
	if got != "/var/log/mullet.log" {
		t.Errorf("GetLogFilePath = %q, want /var/log/mullet.log", got)
	}
}

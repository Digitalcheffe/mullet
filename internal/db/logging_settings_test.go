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

func TestSeedLogFilePathFromEnvOnlySeedsWhenUnset(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SeedLogFilePathFromEnv(sqldb, "/data/logs/mullet.log"); err != nil {
		t.Fatalf("SeedLogFilePathFromEnv: %v", err)
	}
	got, err := GetLogFilePath(sqldb)
	if err != nil {
		t.Fatalf("GetLogFilePath: %v", err)
	}
	if got != "/data/logs/mullet.log" {
		t.Errorf("GetLogFilePath after seed = %q, want /data/logs/mullet.log", got)
	}

	// An admin's later explicit choice -- even reverting to "" (stdout
	// only) -- must not be overwritten by a later seed attempt.
	if err := SetLogFilePath(sqldb, ""); err != nil {
		t.Fatalf("SetLogFilePath: %v", err)
	}
	if err := SeedLogFilePathFromEnv(sqldb, "/data/logs/mullet.log"); err != nil {
		t.Fatalf("SeedLogFilePathFromEnv (second attempt): %v", err)
	}
	got, err = GetLogFilePath(sqldb)
	if err != nil {
		t.Fatalf("GetLogFilePath: %v", err)
	}
	if got != "" {
		t.Errorf("GetLogFilePath after admin reverted to stdout = %q, want empty -- a later seed attempt must not override it", got)
	}
}

func TestSeedLogFilePathFromEnvNoopWhenEmpty(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SeedLogFilePathFromEnv(sqldb, ""); err != nil {
		t.Fatalf("SeedLogFilePathFromEnv: %v", err)
	}
	got, err := GetLogFilePath(sqldb)
	if err != nil {
		t.Fatalf("GetLogFilePath: %v", err)
	}
	if got != "" {
		t.Errorf("GetLogFilePath after no-op seed = %q, want empty", got)
	}
}

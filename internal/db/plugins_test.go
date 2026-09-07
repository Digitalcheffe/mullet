package db

import (
	"errors"
	"testing"
	"time"
)

func TestLoadEnabledPluginInstances(t *testing.T) {
	sqldb := newTestDB(t) // seeds instance id=1, enabled by default

	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, enabled) VALUES (2, 'clock', 'Kitchen Clock', 60, 0)`,
	); err != nil {
		t.Fatalf("seeding disabled instance: %v", err)
	}

	instances, err := LoadEnabledPluginInstances(sqldb)
	if err != nil {
		t.Fatalf("LoadEnabledPluginInstances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("got %d enabled instances, want 1", len(instances))
	}
	if instances[0].ID != 1 || instances[0].PluginID != "openweathermap" {
		t.Errorf("instance = %+v, want ID=1 PluginID=openweathermap", instances[0])
	}
	if instances[0].RefreshInterval != 900*time.Second {
		t.Errorf("RefreshInterval = %v, want 900s", instances[0].RefreshInterval)
	}
}

func TestRecordFetchSuccessAndError(t *testing.T) {
	sqldb := newTestDB(t)

	if err := RecordFetchError(sqldb, 1, errors.New("api key invalid")); err != nil {
		t.Fatalf("RecordFetchError: %v", err)
	}

	var lastError *string
	if err := sqldb.QueryRow(`SELECT last_error FROM data_plugin_instances WHERE id = 1`).Scan(&lastError); err != nil {
		t.Fatalf("reading last_error: %v", err)
	}
	if lastError == nil || *lastError != "api key invalid" {
		t.Errorf("last_error = %v, want %q", lastError, "api key invalid")
	}

	if err := RecordFetchSuccess(sqldb, 1); err != nil {
		t.Fatalf("RecordFetchSuccess: %v", err)
	}

	if err := sqldb.QueryRow(`SELECT last_error FROM data_plugin_instances WHERE id = 1`).Scan(&lastError); err != nil {
		t.Fatalf("reading last_error after success: %v", err)
	}
	if lastError != nil {
		t.Errorf("last_error after success = %v, want nil", *lastError)
	}

	var lastFetchAt *string
	if err := sqldb.QueryRow(`SELECT last_fetch_at FROM data_plugin_instances WHERE id = 1`).Scan(&lastFetchAt); err != nil {
		t.Fatalf("reading last_fetch_at: %v", err)
	}
	if lastFetchAt == nil {
		t.Error("last_fetch_at is nil, want a timestamp")
	}
}

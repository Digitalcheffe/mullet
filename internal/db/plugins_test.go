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

func TestListPluginInstanceStatuses(t *testing.T) {
	sqldb := newTestDB(t) // seeds instance id=1 'openweathermap'/'Home', enabled, no fetch yet

	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, enabled) VALUES (2, 'clock', 'Kitchen Clock', 60, 0)`,
	); err != nil {
		t.Fatalf("seeding disabled instance: %v", err)
	}
	if err := RecordFetchError(sqldb, 1, errors.New("rate limited")); err != nil {
		t.Fatalf("RecordFetchError: %v", err)
	}

	statuses, err := ListPluginInstanceStatuses(sqldb)
	if err != nil {
		t.Fatalf("ListPluginInstanceStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2 (enabled and disabled both included)", len(statuses))
	}

	byID := map[int]PluginInstanceStatus{}
	for _, s := range statuses {
		byID[s.ID] = s
	}

	owm := byID[1]
	if owm.InstanceName != "Home" || owm.PluginID != "openweathermap" || !owm.Enabled {
		t.Errorf("instance 1 = %+v, unexpected values", owm)
	}
	if owm.LastError == nil || *owm.LastError != "rate limited" {
		t.Errorf("instance 1 LastError = %v, want %q", owm.LastError, "rate limited")
	}
	if owm.LastFetchAt == nil {
		t.Error("instance 1 LastFetchAt is nil, want a timestamp")
	}

	clock := byID[2]
	if clock.Enabled {
		t.Error("instance 2 Enabled = true, want false")
	}
	if clock.LastError != nil || clock.LastFetchAt != nil {
		t.Errorf("instance 2 = %+v, want no fetch recorded yet", clock)
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

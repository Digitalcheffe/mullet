package db

import (
	"testing"
	"time"
)

func TestReadShapeUnknownShape(t *testing.T) {
	sqldb := newTestDB(t)

	if _, _, _, err := ReadShape(sqldb, "not_a_shape", nil, nil, nil); err != ErrUnknownShape {
		t.Errorf("ReadShape unknown shape: err = %v, want ErrUnknownShape", err)
	}
}

func TestReadShapeEmptyReturnsEmptySlice(t *testing.T) {
	sqldb := newTestDB(t)

	rows, sources, lastUpdated, err := ReadShape(sqldb, "weather_current", nil, nil, nil)
	if err != nil {
		t.Fatalf("ReadShape: %v", err)
	}
	if rows == nil || len(rows) != 0 {
		t.Errorf("rows = %v, want non-nil empty slice", rows)
	}
	if len(sources) != 0 {
		t.Errorf("sources = %v, want empty", sources)
	}
	if lastUpdated != nil {
		t.Errorf("lastUpdated = %v, want nil", lastUpdated)
	}
}

func TestReadShapeReturnsRowsWithSourceAndLastUpdated(t *testing.T) {
	sqldb := newTestDB(t) // seeds plugin instance id=1, plugin_id='openweathermap'

	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon, fetched_at)
		 VALUES ('current', 1, 21.5, 'Clear', 'clear', '2024-06-01 12:00:00')`,
	); err != nil {
		t.Fatalf("seeding weather row: %v", err)
	}

	rows, sources, lastUpdated, err := ReadShape(sqldb, "weather_current", nil, nil, nil)
	if err != nil {
		t.Fatalf("ReadShape: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want 1 entry", rows)
	}
	if rows[0]["temp"] != 21.5 {
		t.Errorf("rows[0][temp] = %v, want 21.5", rows[0]["temp"])
	}
	if rows[0]["condition"] != "Clear" {
		t.Errorf("rows[0][condition] = %v, want Clear", rows[0]["condition"])
	}
	if _, present := rows[0]["__plugin_id"]; present {
		t.Error("rows[0] should not expose the internal __plugin_id column")
	}
	if len(sources) != 1 || sources[0] != "openweathermap" {
		t.Errorf("sources = %v, want [openweathermap]", sources)
	}
	if lastUpdated == nil || *lastUpdated != "2024-06-01 12:00:00" {
		t.Errorf("lastUpdated = %v, want 2024-06-01 12:00:00", lastUpdated)
	}
}

func TestReadShapeFiltersByPluginInstance(t *testing.T) {
	sqldb := newTestDB(t) // seeds plugin instance id=1

	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (2, 'open-meteo', 'Backyard', 900)`,
	); err != nil {
		t.Fatalf("seeding second instance: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon) VALUES ('current', 1, 20, 'Clear', 'clear')`,
	); err != nil {
		t.Fatalf("seeding instance 1 row: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon) VALUES ('current', 2, 15, 'Rain', 'rain')`,
	); err != nil {
		t.Fatalf("seeding instance 2 row: %v", err)
	}

	instanceTwo := 2
	rows, sources, _, err := ReadShape(sqldb, "weather_current", &instanceTwo, nil, nil)
	if err != nil {
		t.Fatalf("ReadShape: %v", err)
	}
	if len(rows) != 1 || rows[0]["temp"] != 15.0 {
		t.Errorf("rows = %v, want 1 row with temp=15 (instance 2 only)", rows)
	}
	if len(sources) != 1 || sources[0] != "open-meteo" {
		t.Errorf("sources = %v, want [open-meteo]", sources)
	}
}

func TestReadShapeFiltersEventsByTimeRange(t *testing.T) {
	sqldb := newTestDB(t) // seeds plugin instance id=1

	if _, err := sqldb.Exec(
		`INSERT INTO calendars (id, plugin_instance_id, external_id, name) VALUES (1, 1, 'ext-1', 'Family')`,
	); err != nil {
		t.Fatalf("seeding calendar: %v", err)
	}
	events := []struct {
		id, start string
	}{
		{"evt-early", "2024-01-01T00:00:00Z"},
		{"evt-mid", "2024-06-01T00:00:00Z"},
		{"evt-late", "2024-12-01T00:00:00Z"},
	}
	for _, e := range events {
		if _, err := sqldb.Exec(
			`INSERT INTO shape_events (id, plugin_instance_id, calendar_id, title, start) VALUES (?, 1, 1, 'Event', ?)`,
			e.id, e.start,
		); err != nil {
			t.Fatalf("seeding event %s: %v", e.id, err)
		}
	}

	from, _ := time.Parse(time.RFC3339, "2024-03-01T00:00:00Z")
	to, _ := time.Parse(time.RFC3339, "2024-09-01T00:00:00Z")

	rows, _, _, err := ReadShape(sqldb, "events", nil, &from, &to)
	if err != nil {
		t.Fatalf("ReadShape: %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != "evt-mid" {
		t.Errorf("rows = %v, want only evt-mid", rows)
	}
}

func TestSupportsTimeRange(t *testing.T) {
	if !SupportsTimeRange("events") {
		t.Error("SupportsTimeRange(events) = false, want true")
	}
	if SupportsTimeRange("weather_current") {
		t.Error("SupportsTimeRange(weather_current) = true, want false")
	}
}

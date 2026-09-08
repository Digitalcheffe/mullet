package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Migrations seed a clock plugin instance for a real deployment;
	// tests want a clean, deterministic table to assign their own IDs in.
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'openweathermap', 'Home', 900)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}
	return sqldb
}

func TestWriteShapeWeatherCurrentReplaces(t *testing.T) {
	sqldb := newTestDB(t)

	humidity := 55
	first := []any{shapes.WeatherCurrent{ID: "current", Temp: 70.5, Condition: "Sunny", Icon: "sun", Humidity: &humidity}}
	if err := WriteShape(sqldb, "weather_current", 1, first); err != nil {
		t.Fatalf("WriteShape (first): %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_weather_current WHERE plugin_instance_id = 1`).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("after first write: %d rows, want 1", count)
	}

	second := []any{shapes.WeatherCurrent{ID: "current", Temp: 68.0, Condition: "Cloudy", Icon: "cloud"}}
	if err := WriteShape(sqldb, "weather_current", 1, second); err != nil {
		t.Fatalf("WriteShape (second): %v", err)
	}

	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_weather_current WHERE plugin_instance_id = 1`).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("after second write: %d rows, want 1 (replace, not append)", count)
	}

	var condition string
	if err := sqldb.QueryRow(`SELECT condition FROM shape_weather_current WHERE plugin_instance_id = 1`).Scan(&condition); err != nil {
		t.Fatalf("reading condition: %v", err)
	}
	if condition != "Cloudy" {
		t.Errorf("condition = %q, want %q (stale data should be replaced)", condition, "Cloudy")
	}
}

func TestWriteShapeUnknownShape(t *testing.T) {
	sqldb := newTestDB(t)

	if err := WriteShape(sqldb, "not_a_real_shape", 1, nil); err == nil {
		t.Error("WriteShape with unknown shape: expected error, got nil")
	}
}

func TestWriteShapeTypeMismatch(t *testing.T) {
	sqldb := newTestDB(t)

	if err := WriteShape(sqldb, "weather_current", 1, []any{shapes.Task{ID: "wrong-type"}}); err == nil {
		t.Error("WriteShape with mismatched row type: expected error, got nil")
	}
}

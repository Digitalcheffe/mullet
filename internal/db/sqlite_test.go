package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "mullet.db")

	sqldb, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqldb.Close()

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	tables := []string{
		"users", "system_settings", "schema_migrations",
		"data_plugin_instances", "calendars", "task_lists",
		"shape_events", "shape_tasks",
		"shape_weather_current", "shape_weather_forecast",
		"shape_home_devices", "shape_packages",
		"shape_infrastructure", "shape_media_status",
		"themes", "displays", "screens", "cards",
	}
	for _, table := range tables {
		var name string
		err := sqldb.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	var mode string
	if err := sqldb.QueryRow(`PRAGMA journal_mode;`).Scan(&mode); err != nil {
		t.Fatalf("checking journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	// Re-running Migrate on an already-migrated DB must be a no-op, not an error.
	if err := Migrate(sqldb); err != nil {
		t.Fatalf("second Migrate call: %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("counting schema_migrations: %v", err)
	}
	if count != 20 {
		t.Errorf("schema_migrations has %d rows, want 20", count)
	}
}

func TestMigrateSeedsDefaultTheme(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mullet.db")

	sqldb, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqldb.Close()

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	themes, err := ListThemes(sqldb)
	if err != nil {
		t.Fatalf("ListThemes: %v", err)
	}
	if len(themes) != 5 {
		t.Fatalf("themes = %+v, want 5 bundled themes (issue #31, plus Mullet Brand from issue #90)", themes)
	}
	if themes[0].Name != "Dark Glass" || !themes[0].IsDefault {
		t.Errorf("seeded theme = %+v, want (name=Dark Glass, is_default=true)", themes[0])
	}

	var tokens map[string]any
	if err := json.Unmarshal([]byte(themes[0].Tokens), &tokens); err != nil {
		t.Fatalf("seeded theme.Tokens is not valid JSON: %v", err)
	}
	for _, key := range []string{
		"background", "cardBackground", "cardBorder", "cardStyle", "textColor", "accentColor",
		"successColor", "warningColor", "errorColor", "infoColor",
		"fontFamily", "fontSize", "headingFontFamily", "fontSizeSmall", "fontSizeLarge",
		"borderRadius", "opacity", "blur",
	} {
		if _, ok := tokens[key]; !ok {
			t.Errorf("seeded theme.Tokens missing key %q", key)
		}
	}
}

func TestShapeTablesCascadeDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mullet.db")

	sqldb, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqldb.Close()

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}

	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'ics-feed', 'Family', 900)`,
	); err != nil {
		t.Fatalf("inserting plugin instance: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO calendars (id, plugin_instance_id, external_id, name) VALUES (1, 1, 'ext-1', 'Family')`,
	); err != nil {
		t.Fatalf("inserting calendar: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO shape_events (id, plugin_instance_id, calendar_id, title, start) VALUES ('evt-1', 1, 1, 'Dinner', datetime('now'))`,
	); err != nil {
		t.Fatalf("inserting event: %v", err)
	}

	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances WHERE id = 1`); err != nil {
		t.Fatalf("deleting plugin instance: %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_events`).Scan(&count); err != nil {
		t.Fatalf("counting shape_events: %v", err)
	}
	if count != 0 {
		t.Errorf("shape_events has %d rows after cascade delete, want 0", count)
	}
}

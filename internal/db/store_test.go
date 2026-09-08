package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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
	// Migrations seed a clock plugin instance and a default theme for a
	// real deployment; tests want clean, deterministic tables to assign
	// their own IDs in.
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	if _, err := sqldb.Exec(`DELETE FROM themes`); err != nil {
		t.Fatalf("clearing seeded themes: %v", err)
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
	windSpeed := 12.5
	first := []any{shapes.WeatherCurrent{ID: "current", Temp: 70.5, Condition: "Sunny", Icon: "sun", Humidity: &humidity, WindSpeed: &windSpeed}}
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

	var gotWindSpeed float64
	if err := sqldb.QueryRow(`SELECT wind_speed FROM shape_weather_current WHERE plugin_instance_id = 1`).Scan(&gotWindSpeed); err != nil {
		t.Fatalf("reading wind_speed: %v", err)
	}
	if gotWindSpeed != 12.5 {
		t.Errorf("wind_speed = %v, want 12.5", gotWindSpeed)
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

func TestWriteEventsCreatesCalendarAndEvents(t *testing.T) {
	sqldb := newTestDB(t)

	color := "#4285F4"
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	rows := []any{
		shapes.CalendarEvent{
			ID: "evt-1", Title: "Standup", Start: start, End: &end,
			CalendarExternalID: "default", CalendarName: "Family", CalendarColor: &color,
		},
	}
	if err := WriteShape(sqldb, "events", 1, rows); err != nil {
		t.Fatalf("WriteShape: %v", err)
	}

	var calID int
	var name, calColor string
	if err := sqldb.QueryRow(`SELECT id, name, color FROM calendars WHERE plugin_instance_id = 1 AND external_id = 'default'`).Scan(&calID, &name, &calColor); err != nil {
		t.Fatalf("reading calendars row: %v", err)
	}
	if name != "Family" || calColor != "#4285F4" {
		t.Errorf("calendar = (name=%q, color=%q), want (Family, #4285F4)", name, calColor)
	}

	var title string
	var gotCalID int
	if err := sqldb.QueryRow(`SELECT title, calendar_id FROM shape_events WHERE id = 'evt-1' AND plugin_instance_id = 1`).Scan(&title, &gotCalID); err != nil {
		t.Fatalf("reading shape_events row: %v", err)
	}
	if title != "Standup" || gotCalID != calID {
		t.Errorf("event = (title=%q, calendar_id=%d), want (Standup, %d)", title, gotCalID, calID)
	}
}

func TestWriteEventsReplacesWithinCalendar(t *testing.T) {
	sqldb := newTestDB(t)

	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	first := []any{shapes.CalendarEvent{ID: "evt-1", Title: "Old", Start: start, CalendarExternalID: "default", CalendarName: "Family"}}
	if err := WriteShape(sqldb, "events", 1, first); err != nil {
		t.Fatalf("WriteShape (first): %v", err)
	}

	second := []any{shapes.CalendarEvent{ID: "evt-2", Title: "New", Start: start, CalendarExternalID: "default", CalendarName: "Family"}}
	if err := WriteShape(sqldb, "events", 1, second); err != nil {
		t.Fatalf("WriteShape (second): %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_events WHERE plugin_instance_id = 1`).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("after second write: %d rows, want 1 (replace, not append)", count)
	}

	var title string
	if err := sqldb.QueryRow(`SELECT title FROM shape_events WHERE plugin_instance_id = 1`).Scan(&title); err != nil {
		t.Fatalf("reading title: %v", err)
	}
	if title != "New" {
		t.Errorf("title = %q, want %q (stale event should be replaced)", title, "New")
	}

	// The old calendar row is reused (upserted), not recreated.
	var calCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM calendars WHERE plugin_instance_id = 1`).Scan(&calCount); err != nil {
		t.Fatalf("counting calendars: %v", err)
	}
	if calCount != 1 {
		t.Errorf("calendars count = %d, want 1", calCount)
	}
}

func TestWriteEventsScopesReplaceToItsOwnCalendar(t *testing.T) {
	sqldb := newTestDB(t)

	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	both := []any{
		shapes.CalendarEvent{ID: "work-1", Title: "Meeting", Start: start, CalendarExternalID: "work", CalendarName: "Work"},
		shapes.CalendarEvent{ID: "family-1", Title: "Dinner", Start: start, CalendarExternalID: "family", CalendarName: "Family"},
	}
	if err := WriteShape(sqldb, "events", 1, both); err != nil {
		t.Fatalf("WriteShape (both): %v", err)
	}

	// A later write touching only "work" must not remove "family"'s event.
	workOnly := []any{
		shapes.CalendarEvent{ID: "work-2", Title: "Standup", Start: start, CalendarExternalID: "work", CalendarName: "Work"},
	}
	if err := WriteShape(sqldb, "events", 1, workOnly); err != nil {
		t.Fatalf("WriteShape (work only): %v", err)
	}

	var familyCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_events WHERE id = 'family-1' AND plugin_instance_id = 1`).Scan(&familyCount); err != nil {
		t.Fatalf("counting family events: %v", err)
	}
	if familyCount != 1 {
		t.Error("family-1 event was removed by a write scoped to the work calendar")
	}

	var workCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_events WHERE plugin_instance_id = 1 AND calendar_id = (SELECT id FROM calendars WHERE external_id = 'work' AND plugin_instance_id = 1)`).Scan(&workCount); err != nil {
		t.Fatalf("counting work events: %v", err)
	}
	if workCount != 1 {
		t.Errorf("work calendar has %d events, want 1 (work-1 replaced by work-2)", workCount)
	}
}

func TestWriteEventsMissingCalendarExternalID(t *testing.T) {
	sqldb := newTestDB(t)

	rows := []any{shapes.CalendarEvent{ID: "evt-1", Title: "No calendar", Start: time.Now()}}
	if err := WriteShape(sqldb, "events", 1, rows); err == nil {
		t.Error("WriteShape with no CalendarExternalID: expected error, got nil")
	}
}

func TestWriteTasksCreatesTaskListAndTasks(t *testing.T) {
	sqldb := newTestDB(t)

	due := "2026-01-15"
	rows := []any{
		shapes.Task{
			ID: "task-1", Title: "Buy milk", DueDate: &due, SortOrder: 1,
			TaskListExternalID: "default", TaskListName: "Tasks",
		},
	}
	if err := WriteShape(sqldb, "tasks", 1, rows); err != nil {
		t.Fatalf("WriteShape: %v", err)
	}

	var listID int
	var name string
	if err := sqldb.QueryRow(`SELECT id, name FROM task_lists WHERE plugin_instance_id = 1 AND external_id = 'default'`).Scan(&listID, &name); err != nil {
		t.Fatalf("reading task_lists row: %v", err)
	}
	if name != "Tasks" {
		t.Errorf("task list name = %q, want Tasks", name)
	}

	var title string
	var gotListID int
	var completed int
	if err := sqldb.QueryRow(`SELECT title, task_list_id, completed FROM shape_tasks WHERE id = 'task-1' AND plugin_instance_id = 1`).Scan(&title, &gotListID, &completed); err != nil {
		t.Fatalf("reading shape_tasks row: %v", err)
	}
	if title != "Buy milk" || gotListID != listID || completed != 0 {
		t.Errorf("task = (title=%q, task_list_id=%d, completed=%d), want (Buy milk, %d, 0)", title, gotListID, completed, listID)
	}
}

func TestWriteTasksReplacesWithinTaskList(t *testing.T) {
	sqldb := newTestDB(t)

	first := []any{shapes.Task{ID: "task-1", Title: "Old", TaskListExternalID: "default", TaskListName: "Tasks"}}
	if err := WriteShape(sqldb, "tasks", 1, first); err != nil {
		t.Fatalf("WriteShape (first): %v", err)
	}

	second := []any{shapes.Task{ID: "task-2", Title: "New", TaskListExternalID: "default", TaskListName: "Tasks"}}
	if err := WriteShape(sqldb, "tasks", 1, second); err != nil {
		t.Fatalf("WriteShape (second): %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_tasks WHERE plugin_instance_id = 1`).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("after second write: %d rows, want 1 (replace, not append)", count)
	}

	var title string
	if err := sqldb.QueryRow(`SELECT title FROM shape_tasks WHERE plugin_instance_id = 1`).Scan(&title); err != nil {
		t.Fatalf("reading title: %v", err)
	}
	if title != "New" {
		t.Errorf("title = %q, want %q (stale task should be replaced)", title, "New")
	}

	var listCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM task_lists WHERE plugin_instance_id = 1`).Scan(&listCount); err != nil {
		t.Fatalf("counting task_lists: %v", err)
	}
	if listCount != 1 {
		t.Errorf("task_lists count = %d, want 1 (reused, not recreated)", listCount)
	}
}

func TestWriteTasksScopesReplaceToItsOwnTaskList(t *testing.T) {
	sqldb := newTestDB(t)

	both := []any{
		shapes.Task{ID: "work-1", Title: "Ship it", TaskListExternalID: "work", TaskListName: "Work"},
		shapes.Task{ID: "home-1", Title: "Mow lawn", TaskListExternalID: "home", TaskListName: "Home"},
	}
	if err := WriteShape(sqldb, "tasks", 1, both); err != nil {
		t.Fatalf("WriteShape (both): %v", err)
	}

	workOnly := []any{shapes.Task{ID: "work-2", Title: "Ship it again", TaskListExternalID: "work", TaskListName: "Work"}}
	if err := WriteShape(sqldb, "tasks", 1, workOnly); err != nil {
		t.Fatalf("WriteShape (work only): %v", err)
	}

	var homeCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_tasks WHERE id = 'home-1' AND plugin_instance_id = 1`).Scan(&homeCount); err != nil {
		t.Fatalf("counting home tasks: %v", err)
	}
	if homeCount != 1 {
		t.Error("home-1 task was removed by a write scoped to the work task list")
	}

	var workCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_tasks WHERE plugin_instance_id = 1 AND task_list_id = (SELECT id FROM task_lists WHERE external_id = 'work' AND plugin_instance_id = 1)`).Scan(&workCount); err != nil {
		t.Fatalf("counting work tasks: %v", err)
	}
	if workCount != 1 {
		t.Errorf("work task list has %d tasks, want 1 (work-1 replaced by work-2)", workCount)
	}
}

func TestWriteTasksPriorityDefaultsToNormal(t *testing.T) {
	sqldb := newTestDB(t)

	rows := []any{
		shapes.Task{ID: "task-1", Title: "No priority set", TaskListExternalID: "default", TaskListName: "Tasks"},
		shapes.Task{ID: "task-2", Title: "High priority", Priority: "high", TaskListExternalID: "default", TaskListName: "Tasks"},
	}
	if err := WriteShape(sqldb, "tasks", 1, rows); err != nil {
		t.Fatalf("WriteShape: %v", err)
	}

	var p1, p2 string
	if err := sqldb.QueryRow(`SELECT priority FROM shape_tasks WHERE id = 'task-1' AND plugin_instance_id = 1`).Scan(&p1); err != nil {
		t.Fatalf("reading task-1 priority: %v", err)
	}
	if p1 != "normal" {
		t.Errorf("task-1 priority = %q, want normal (default for unset Priority)", p1)
	}
	if err := sqldb.QueryRow(`SELECT priority FROM shape_tasks WHERE id = 'task-2' AND plugin_instance_id = 1`).Scan(&p2); err != nil {
		t.Fatalf("reading task-2 priority: %v", err)
	}
	if p2 != "high" {
		t.Errorf("task-2 priority = %q, want high (explicitly set)", p2)
	}
}

func TestWriteTasksMissingTaskListExternalID(t *testing.T) {
	sqldb := newTestDB(t)

	rows := []any{shapes.Task{ID: "task-1", Title: "No list"}}
	if err := WriteShape(sqldb, "tasks", 1, rows); err == nil {
		t.Error("WriteShape with no TaskListExternalID: expected error, got nil")
	}
}

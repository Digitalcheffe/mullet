package db

import (
	"database/sql"
	"fmt"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// shapeWriter replaces every row for pluginInstanceID in a shape table
// with rows, within tx. Each writer type-asserts rows to its shape's
// concrete Go struct (see internal/shapes) -- there is no reflection or
// map[string]any involved.
type shapeWriter func(tx *sql.Tx, pluginInstanceID int, rows []any) error

// shapeWriters covers the shapes whose write scope is just
// plugin_instance_id (Replace strategy), plus events and tasks, which are
// Upsert strategy scoped by calendar_id/task_list_id and need a plugin
// with entity discovery to populate the calendars/task_lists metadata
// tables first (icsfeed for events; msgraphcalendar/msgraphtodo for both).
var shapeWriters = map[string]shapeWriter{
	"weather_current":  writeWeatherCurrent,
	"weather_forecast": writeWeatherForecast,
	"home_devices":     writeHomeDevices,
	"packages":         writePackages,
	"infrastructure":   writeInfrastructure,
	"media_status":     writeMediaStatus,
	"events":           writeEvents,
	"tasks":            writeTasks,
}

// WriteShape replaces all rows for pluginInstanceID in the table for
// shape with rows, in a single transaction.
func WriteShape(sqldb *sql.DB, shape string, pluginInstanceID int, rows []any) error {
	writer, ok := shapeWriters[shape]
	if !ok {
		return fmt.Errorf("no writer registered for shape %q", shape)
	}

	tx, err := sqldb.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := writer(tx, pluginInstanceID, rows); err != nil {
		return err
	}

	return tx.Commit()
}

func writeWeatherCurrent(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_weather_current WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_weather_current: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_weather_current
			(id, plugin_instance_id, temp, feels_like, condition, icon, humidity, high, low, sunrise, sunset, wind_speed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_weather_current insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		w, ok := row.(shapes.WeatherCurrent)
		if !ok {
			return fmt.Errorf("weather_current writer: expected shapes.WeatherCurrent, got %T", row)
		}
		if _, err := stmt.Exec(w.ID, pluginInstanceID, w.Temp, w.FeelsLike, w.Condition, w.Icon, w.Humidity, w.High, w.Low, w.Sunrise, w.Sunset, w.WindSpeed); err != nil {
			return fmt.Errorf("inserting shape_weather_current row %q: %w", w.ID, err)
		}
	}
	return nil
}

func writeWeatherForecast(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_weather_forecast WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_weather_forecast: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_weather_forecast
			(id, plugin_instance_id, date, high, low, condition, icon, precip_chance)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_weather_forecast insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		w, ok := row.(shapes.WeatherForecast)
		if !ok {
			return fmt.Errorf("weather_forecast writer: expected shapes.WeatherForecast, got %T", row)
		}
		if _, err := stmt.Exec(w.ID, pluginInstanceID, w.Date, w.High, w.Low, w.Condition, w.Icon, w.PrecipChance); err != nil {
			return fmt.Errorf("inserting shape_weather_forecast row %q: %w", w.ID, err)
		}
	}
	return nil
}

func writeHomeDevices(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_home_devices WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_home_devices: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_home_devices (id, plugin_instance_id, name, area, device_type, state)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_home_devices insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		d, ok := row.(shapes.HomeDevice)
		if !ok {
			return fmt.Errorf("home_devices writer: expected shapes.HomeDevice, got %T", row)
		}
		if _, err := stmt.Exec(d.ID, pluginInstanceID, d.Name, d.Area, d.DeviceType, d.State); err != nil {
			return fmt.Errorf("inserting shape_home_devices row %q: %w", d.ID, err)
		}
	}
	return nil
}

func writePackages(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_packages WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_packages: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_packages (id, plugin_instance_id, carrier, description, status, eta, tracking_url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_packages insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		p, ok := row.(shapes.Package)
		if !ok {
			return fmt.Errorf("packages writer: expected shapes.Package, got %T", row)
		}
		if _, err := stmt.Exec(p.ID, pluginInstanceID, p.Carrier, p.Description, p.Status, p.ETA, p.TrackingURL); err != nil {
			return fmt.Errorf("inserting shape_packages row %q: %w", p.ID, err)
		}
	}
	return nil
}

func writeInfrastructure(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_infrastructure WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_infrastructure: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_infrastructure (id, plugin_instance_id, name, status, cpu_percent, memory_percent, disk_percent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_infrastructure insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		s, ok := row.(shapes.InfraService)
		if !ok {
			return fmt.Errorf("infrastructure writer: expected shapes.InfraService, got %T", row)
		}
		if _, err := stmt.Exec(s.ID, pluginInstanceID, s.Name, s.Status, s.CPUPercent, s.MemoryPercent, s.DiskPercent); err != nil {
			return fmt.Errorf("inserting shape_infrastructure row %q: %w", s.ID, err)
		}
	}
	return nil
}

// writeEvents implements the events contract's Upsert write strategy
// (see docs/architecture_1.md "Write Path"): rows arrive with a
// plugin-assigned CalendarExternalID/Name/Color rather than a real
// calendars.id, since a plugin has no DB access to resolve one itself.
// For each distinct external ID, this upserts a calendars row (entity
// discovery), then replaces that calendar's shape_events rows scoped
// by (plugin_instance_id, calendar_id) -- so a plugin instance fetching
// multiple calendars in one cycle never clobbers a calendar's data with
// another's.
//
// Known limitation: a calendar only gets cleared/refreshed when at least
// one of its rows shows up in this call, since an empty rows slice
// carries no CalendarExternalID to act on. A calendar whose event count
// drops to exactly zero (rather than just shrinking) keeps its last-known
// events until its next non-empty fetch. Acceptable for now -- retention
// /cleanup of stale rows is a separate, not-yet-built concern (see the
// architecture doc's "Retention / Cleanup" section).
func writeEvents(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	events := make([]shapes.CalendarEvent, len(rows))
	for i, row := range rows {
		e, ok := row.(shapes.CalendarEvent)
		if !ok {
			return fmt.Errorf("events writer: expected shapes.CalendarEvent, got %T", row)
		}
		if e.CalendarExternalID == "" {
			return fmt.Errorf("events writer: event %q has no CalendarExternalID", e.ID)
		}
		events[i] = e
	}

	byCalendar := map[string][]shapes.CalendarEvent{}
	var order []string
	for _, e := range events {
		if _, seen := byCalendar[e.CalendarExternalID]; !seen {
			order = append(order, e.CalendarExternalID)
		}
		byCalendar[e.CalendarExternalID] = append(byCalendar[e.CalendarExternalID], e)
	}

	upsertCalendar, err := tx.Prepare(`
		INSERT INTO calendars (plugin_instance_id, external_id, name, color)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(plugin_instance_id, external_id) DO UPDATE SET name = excluded.name, color = excluded.color
	`)
	if err != nil {
		return fmt.Errorf("preparing calendars upsert: %w", err)
	}
	defer upsertCalendar.Close()

	selectCalendarID, err := tx.Prepare(`SELECT id FROM calendars WHERE plugin_instance_id = ? AND external_id = ?`)
	if err != nil {
		return fmt.Errorf("preparing calendars lookup: %w", err)
	}
	defer selectCalendarID.Close()

	deleteEvents, err := tx.Prepare(`DELETE FROM shape_events WHERE plugin_instance_id = ? AND calendar_id = ?`)
	if err != nil {
		return fmt.Errorf("preparing shape_events delete: %w", err)
	}
	defer deleteEvents.Close()

	insertEvent, err := tx.Prepare(`
		INSERT INTO shape_events
			(id, plugin_instance_id, calendar_id, title, start, end, all_day, location, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_events insert: %w", err)
	}
	defer insertEvent.Close()

	for _, externalID := range order {
		calEvents := byCalendar[externalID]
		name := calEvents[0].CalendarName
		var color *string
		if calEvents[0].CalendarColor != nil {
			color = calEvents[0].CalendarColor
		}
		if _, err := upsertCalendar.Exec(pluginInstanceID, externalID, name, color); err != nil {
			return fmt.Errorf("upserting calendar %q: %w", externalID, err)
		}

		var calendarID int
		if err := selectCalendarID.QueryRow(pluginInstanceID, externalID).Scan(&calendarID); err != nil {
			return fmt.Errorf("looking up calendar %q: %w", externalID, err)
		}

		if _, err := deleteEvents.Exec(pluginInstanceID, calendarID); err != nil {
			return fmt.Errorf("clearing shape_events for calendar %q: %w", externalID, err)
		}

		for _, e := range calEvents {
			if _, err := insertEvent.Exec(e.ID, pluginInstanceID, calendarID, e.Title, e.Start, e.End, e.AllDay, e.Location, e.Description); err != nil {
				return fmt.Errorf("inserting shape_events row %q: %w", e.ID, err)
			}
		}
	}

	return nil
}

func writeTasks(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	tasks := make([]shapes.Task, len(rows))
	for i, row := range rows {
		t, ok := row.(shapes.Task)
		if !ok {
			return fmt.Errorf("tasks writer: expected shapes.Task, got %T", row)
		}
		if t.TaskListExternalID == "" {
			return fmt.Errorf("tasks writer: task %q has no TaskListExternalID", t.ID)
		}
		tasks[i] = t
	}

	byList := map[string][]shapes.Task{}
	var order []string
	for _, t := range tasks {
		if _, seen := byList[t.TaskListExternalID]; !seen {
			order = append(order, t.TaskListExternalID)
		}
		byList[t.TaskListExternalID] = append(byList[t.TaskListExternalID], t)
	}

	upsertTaskList, err := tx.Prepare(`
		INSERT INTO task_lists (plugin_instance_id, external_id, name)
		VALUES (?, ?, ?)
		ON CONFLICT(plugin_instance_id, external_id) DO UPDATE SET name = excluded.name
	`)
	if err != nil {
		return fmt.Errorf("preparing task_lists upsert: %w", err)
	}
	defer upsertTaskList.Close()

	selectTaskListID, err := tx.Prepare(`SELECT id FROM task_lists WHERE plugin_instance_id = ? AND external_id = ?`)
	if err != nil {
		return fmt.Errorf("preparing task_lists lookup: %w", err)
	}
	defer selectTaskListID.Close()

	deleteTasks, err := tx.Prepare(`DELETE FROM shape_tasks WHERE plugin_instance_id = ? AND task_list_id = ?`)
	if err != nil {
		return fmt.Errorf("preparing shape_tasks delete: %w", err)
	}
	defer deleteTasks.Close()

	insertTask, err := tx.Prepare(`
		INSERT INTO shape_tasks
			(id, plugin_instance_id, task_list_id, title, completed, due_date, sort_order, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_tasks insert: %w", err)
	}
	defer insertTask.Close()

	for _, externalID := range order {
		listTasks := byList[externalID]
		name := listTasks[0].TaskListName
		if _, err := upsertTaskList.Exec(pluginInstanceID, externalID, name); err != nil {
			return fmt.Errorf("upserting task list %q: %w", externalID, err)
		}

		var taskListID int
		if err := selectTaskListID.QueryRow(pluginInstanceID, externalID).Scan(&taskListID); err != nil {
			return fmt.Errorf("looking up task list %q: %w", externalID, err)
		}

		if _, err := deleteTasks.Exec(pluginInstanceID, taskListID); err != nil {
			return fmt.Errorf("clearing shape_tasks for task list %q: %w", externalID, err)
		}

		for _, t := range listTasks {
			priority := t.Priority
			if priority == "" {
				priority = "normal"
			}
			if _, err := insertTask.Exec(t.ID, pluginInstanceID, taskListID, t.Title, t.Completed, t.DueDate, t.SortOrder, priority); err != nil {
				return fmt.Errorf("inserting shape_tasks row %q: %w", t.ID, err)
			}
		}
	}

	return nil
}

func writeMediaStatus(tx *sql.Tx, pluginInstanceID int, rows []any) error {
	if _, err := tx.Exec(`DELETE FROM shape_media_status WHERE plugin_instance_id = ?`, pluginInstanceID); err != nil {
		return fmt.Errorf("clearing shape_media_status: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO shape_media_status (id, plugin_instance_id, player_name, is_playing, title, artist, album_art_url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing shape_media_status insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		m, ok := row.(shapes.MediaStatus)
		if !ok {
			return fmt.Errorf("media_status writer: expected shapes.MediaStatus, got %T", row)
		}
		if _, err := stmt.Exec(m.ID, pluginInstanceID, m.PlayerName, m.IsPlaying, m.Title, m.Artist, m.AlbumArtURL); err != nil {
			return fmt.Errorf("inserting shape_media_status row %q: %w", m.ID, err)
		}
	}
	return nil
}

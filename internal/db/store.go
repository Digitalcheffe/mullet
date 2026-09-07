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
// plugin_instance_id (Replace strategy). events and tasks are Upsert
// strategy scoped by calendar_id/task_list_id, which requires metadata
// table rows (calendars/task_lists) that don't exist until a plugin with
// entity discovery is wired up (issues #16, #24-26); their writers land
// with that work.
var shapeWriters = map[string]shapeWriter{
	"weather_current":  writeWeatherCurrent,
	"weather_forecast": writeWeatherForecast,
	"home_devices":     writeHomeDevices,
	"packages":         writePackages,
	"infrastructure":   writeInfrastructure,
	"media_status":     writeMediaStatus,
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
			(id, plugin_instance_id, temp, feels_like, condition, icon, humidity, high, low, sunrise, sunset)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		if _, err := stmt.Exec(w.ID, pluginInstanceID, w.Temp, w.FeelsLike, w.Condition, w.Icon, w.Humidity, w.High, w.Low, w.Sunrise, w.Sunset); err != nil {
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

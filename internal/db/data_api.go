package db

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrUnknownShape is returned by ReadShape for a shape name with no
// backing table.
var ErrUnknownShape = errors.New("unknown shape")

// shapeTables maps every shape name -- framework or custom, written-to
// yet or not -- to its backing shape_* table. This is deliberately wider
// than shapeWriters (store.go): a shape can be readable (its table
// exists from migration 002) before any plugin writes to it, e.g.
// events/tasks before issue #16/#24-26 land. GET /api/data/{shape} just
// serves whatever's in the table, empty or not.
var shapeTables = map[string]string{
	"events":           "shape_events",
	"tasks":            "shape_tasks",
	"weather_current":  "shape_weather_current",
	"weather_forecast": "shape_weather_forecast",
	"home_devices":     "shape_home_devices",
	"packages":         "shape_packages",
	"infrastructure":   "shape_infrastructure",
	"media_status":     "shape_media_status",
}

// shapeTimeColumns names the column a shape's optional from/to range
// filter applies to. Only "events" has one for now, matching the
// issue's own example; add more here if a future shape needs it.
var shapeTimeColumns = map[string]string{
	"events": "start",
}

// IsValidShape reports whether shape is a recognized data shape.
func IsValidShape(shape string) bool {
	_, ok := shapeTables[shape]
	return ok
}

// SupportsTimeRange reports whether shape accepts the from/to filter.
func SupportsTimeRange(shape string) bool {
	_, ok := shapeTimeColumns[shape]
	return ok
}

// ReadShape queries shape's table, optionally scoped to one plugin
// instance and/or (for shapes in shapeTimeColumns) a time range. It
// returns each row as a JSON-friendly column-name-to-value map, the
// distinct plugin_id(s) that produced them, and the most recent
// fetched_at across the returned rows (nil if there are none).
func ReadShape(sqldb *sql.DB, shape string, pluginInstanceID *int, from, to *time.Time) (rows []map[string]any, sources []string, lastUpdated *string, err error) {
	table, ok := shapeTables[shape]
	if !ok {
		return nil, nil, nil, ErrUnknownShape
	}

	query := fmt.Sprintf(
		`SELECT s.*, dpi.plugin_id AS __plugin_id FROM %s s JOIN data_plugin_instances dpi ON dpi.id = s.plugin_instance_id`,
		table,
	)
	var conditions []string
	var args []any

	if pluginInstanceID != nil {
		conditions = append(conditions, "s.plugin_instance_id = ?")
		args = append(args, *pluginInstanceID)
	}

	if timeCol, ok := shapeTimeColumns[shape]; ok {
		if from != nil {
			conditions = append(conditions, fmt.Sprintf("s.%s >= ?", timeCol))
			args = append(args, from.UTC().Format(time.RFC3339))
		}
		if to != nil {
			conditions = append(conditions, fmt.Sprintf("s.%s <= ?", timeCol))
			args = append(args, to.UTC().Format(time.RFC3339))
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	sqlRows, err := sqldb.Query(query, args...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("querying shape %q: %w", shape, err)
	}
	defer sqlRows.Close()

	cols, err := sqlRows.Columns()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading columns for shape %q: %w", shape, err)
	}

	result := []map[string]any{}
	sourceSet := map[string]bool{}

	for sqlRows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := sqlRows.Scan(ptrs...); err != nil {
			return nil, nil, nil, fmt.Errorf("scanning shape %q row: %w", shape, err)
		}

		row := make(map[string]any, len(cols)-1)
		var pluginID string
		for i, col := range cols {
			v := normalizeSQLValue(vals[i])
			if col == "__plugin_id" {
				if s, ok := v.(string); ok {
					pluginID = s
				}
				continue
			}
			row[col] = v
			if col == "fetched_at" {
				if s, ok := v.(string); ok {
					if lastUpdated == nil || s > *lastUpdated {
						lastUpdated = &s
					}
				}
			}
		}
		result = append(result, row)
		if pluginID != "" {
			sourceSet[pluginID] = true
		}
	}
	if err := sqlRows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("iterating shape %q rows: %w", shape, err)
	}

	sources = make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	return result, sources, lastUpdated, nil
}

// normalizeSQLValue converts driver-specific scan results into their
// natural JSON-friendly Go type: []byte (TEXT columns) to string, and
// time.Time (this driver auto-converts DATETIME-affinity columns like
// fetched_at/start) to the same "YYYY-MM-DD HH:MM:SS" string form
// SQLite's own CURRENT_TIMESTAMP produces, so a shape's timestamp columns
// look the same in JSON regardless of whether the row was inserted by
// the driver's time.Time binding or by a raw SQL literal.
func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format("2006-01-02 15:04:05")
	default:
		return v
	}
}

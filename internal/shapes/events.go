// Package shapes defines the typed data contracts shared between data
// plugins and UI plugins. The framework owns these shapes; plugins write
// to them (data plugins) or read from them (UI plugins), never define
// their own copy of a framework contract.
package shapes

import "time"

// CalendarEvent represents a calendar event. Written by calendar data
// plugins, consumed by calendar/agenda UI plugins.
// Related metadata: calendars table (name, color, enabled).
type CalendarEvent struct {
	ID               string     `db:"id"`
	PluginInstanceID int        `db:"plugin_instance_id"`
	CalendarID       int        `db:"calendar_id"`
	Title            string     `db:"title"`
	Start            time.Time  `db:"start"`
	End              *time.Time `db:"end"`
	AllDay           bool       `db:"all_day"`
	Location         *string    `db:"location"`
	Description      *string    `db:"description"`

	// CalendarExternalID/CalendarName/CalendarColor identify the source
	// calendar this event belongs to. A plugin has no DB access to
	// resolve a real calendars.id itself, so it sets these instead;
	// the events writer (internal/db/store.go) upserts a calendars row
	// per distinct CalendarExternalID and fills in CalendarID before
	// insert. Not stored on shape_events itself -- write-time only.
	CalendarExternalID string
	CalendarName       string
	CalendarColor      *string
}

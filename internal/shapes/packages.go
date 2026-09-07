package shapes

import "time"

// Package represents an incoming delivery. Written by package-tracking
// data plugins, consumed by the package-tracker UI plugin.
// Custom contract -- no related metadata table.
type Package struct {
	ID               string     `db:"id"`
	PluginInstanceID int        `db:"plugin_instance_id"`
	Carrier          string     `db:"carrier"`
	Description      *string    `db:"description"`
	Status           string     `db:"status"`
	ETA              *time.Time `db:"eta"`
	TrackingURL      *string    `db:"tracking_url"`
}

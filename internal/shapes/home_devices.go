package shapes

// HomeDevice represents a smart-home device's current state (light, lock,
// door, garage, sensor, climate). Written by home integration plugins
// (e.g. Home Assistant), consumed by the home-status UI plugin.
// Custom contract -- no related metadata table.
type HomeDevice struct {
	ID               string  `db:"id"`
	PluginInstanceID int     `db:"plugin_instance_id"`
	Name             string  `db:"name"`
	Area             *string `db:"area"`
	DeviceType       string  `db:"device_type"`
	State            string  `db:"state"`
	// Icon is a same-origin URL to this entity's own Home
	// Assistant-reported icon, already resolved and cached server-side
	// (see internal/mdiicons) -- nil if it has none, or its "mdi:xxx"
	// name didn't resolve to a known icon.
	Icon *string `db:"icon"`
}

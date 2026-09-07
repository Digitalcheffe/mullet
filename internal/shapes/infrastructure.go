package shapes

// InfraService represents the status of a monitored server or container.
// Written by infrastructure data plugins (e.g. Portainer, PRTG), consumed
// by the server-health UI plugin.
// Custom contract -- no related metadata table.
type InfraService struct {
	ID               string   `db:"id"`
	PluginInstanceID int      `db:"plugin_instance_id"`
	Name             string   `db:"name"`
	Status           string   `db:"status"`
	CPUPercent       *float64 `db:"cpu_percent"`
	MemoryPercent    *float64 `db:"memory_percent"`
	DiskPercent      *float64 `db:"disk_percent"`
}

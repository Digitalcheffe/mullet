package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned by update/delete operations that target a row
// which doesn't exist.
var ErrNotFound = errors.New("not found")

// PluginInstance is a configured data plugin instance loaded from
// data_plugin_instances. Config is the raw JSON blob from the setup
// wizard, passed to DataPlugin.Configure before the plugin is scheduled.
type PluginInstance struct {
	ID              int
	PluginID        string
	Config          string
	RefreshInterval time.Duration
}

// LoadEnabledPluginInstances returns every enabled plugin instance.
func LoadEnabledPluginInstances(sqldb *sql.DB) ([]PluginInstance, error) {
	rows, err := sqldb.Query(`SELECT id, plugin_id, config, refresh_seconds FROM data_plugin_instances WHERE enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("loading plugin instances: %w", err)
	}
	defer rows.Close()

	var instances []PluginInstance
	for rows.Next() {
		var inst PluginInstance
		var refreshSeconds int
		if err := rows.Scan(&inst.ID, &inst.PluginID, &inst.Config, &refreshSeconds); err != nil {
			return nil, fmt.Errorf("scanning plugin instance: %w", err)
		}
		inst.RefreshInterval = time.Duration(refreshSeconds) * time.Second
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

// PluginInstanceStatus is a configured plugin instance's full record,
// used by both the admin dashboard's Plugin Status panel (which ignores
// Config) and the plugin management page (which needs Config to prefill
// the edit form).
type PluginInstanceStatus struct {
	ID              int
	PluginID        string
	InstanceName    string
	Config          string
	RefreshInterval time.Duration
	Enabled         bool
	LastFetchAt     *string
	LastError       *string
}

const pluginInstanceStatusColumns = `id, plugin_id, instance_name, config, refresh_seconds, enabled, last_fetch_at, last_error`

func scanPluginInstanceStatus(row interface{ Scan(...any) error }) (PluginInstanceStatus, error) {
	var s PluginInstanceStatus
	var refreshSeconds int
	var enabled int
	if err := row.Scan(&s.ID, &s.PluginID, &s.InstanceName, &s.Config, &refreshSeconds, &enabled, &s.LastFetchAt, &s.LastError); err != nil {
		return PluginInstanceStatus{}, err
	}
	s.RefreshInterval = time.Duration(refreshSeconds) * time.Second
	s.Enabled = enabled != 0
	return s, nil
}

// ListPluginInstanceStatuses returns every plugin instance (enabled or
// not), most recently created first.
func ListPluginInstanceStatuses(sqldb *sql.DB) ([]PluginInstanceStatus, error) {
	rows, err := sqldb.Query(`SELECT ` + pluginInstanceStatusColumns + ` FROM data_plugin_instances ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing plugin instance statuses: %w", err)
	}
	defer rows.Close()

	var statuses []PluginInstanceStatus
	for rows.Next() {
		s, err := scanPluginInstanceStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning plugin instance status: %w", err)
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

// GetPluginInstance returns one plugin instance by ID, or ErrNotFound.
func GetPluginInstance(sqldb *sql.DB, id int) (PluginInstanceStatus, error) {
	row := sqldb.QueryRow(`SELECT `+pluginInstanceStatusColumns+` FROM data_plugin_instances WHERE id = ?`, id)
	s, err := scanPluginInstanceStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PluginInstanceStatus{}, ErrNotFound
	}
	if err != nil {
		return PluginInstanceStatus{}, fmt.Errorf("getting plugin instance %d: %w", id, err)
	}
	return s, nil
}

// CreatePluginInstance inserts a new plugin instance and returns its ID.
func CreatePluginInstance(sqldb *sql.DB, pluginID, instanceName string, refreshSeconds int, enabled bool, config string) (int, error) {
	result, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (plugin_id, instance_name, refresh_seconds, enabled, config) VALUES (?, ?, ?, ?, ?)`,
		pluginID, instanceName, refreshSeconds, boolToInt(enabled), config,
	)
	if err != nil {
		return 0, fmt.Errorf("creating plugin instance: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading new plugin instance id: %w", err)
	}
	return int(id), nil
}

// UpdatePluginInstance overwrites an existing plugin instance's editable
// fields. Returns ErrNotFound if id doesn't exist.
func UpdatePluginInstance(sqldb *sql.DB, id int, instanceName string, refreshSeconds int, enabled bool, config string) error {
	result, err := sqldb.Exec(
		`UPDATE data_plugin_instances SET instance_name = ?, refresh_seconds = ?, enabled = ?, config = ? WHERE id = ?`,
		instanceName, refreshSeconds, boolToInt(enabled), config, id,
	)
	if err != nil {
		return fmt.Errorf("updating plugin instance %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeletePluginInstance removes a plugin instance (and, via ON DELETE
// CASCADE, any shape rows scoped to it). Returns ErrNotFound if id
// doesn't exist.
func DeletePluginInstance(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM data_plugin_instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting plugin instance %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

func checkRowsAffected(result sql.Result, id int) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking result for instance %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// RecordFetchSuccess marks instanceID as freshly fetched with no error.
func RecordFetchSuccess(sqldb *sql.DB, instanceID int) error {
	_, err := sqldb.Exec(
		`UPDATE data_plugin_instances SET last_fetch_at = CURRENT_TIMESTAMP, last_error = NULL WHERE id = ?`,
		instanceID,
	)
	if err != nil {
		return fmt.Errorf("recording fetch success for instance %d: %w", instanceID, err)
	}
	return nil
}

// RecordFetchError marks instanceID's most recent fetch attempt as
// failed, recording the error message.
func RecordFetchError(sqldb *sql.DB, instanceID int, fetchErr error) error {
	_, err := sqldb.Exec(
		`UPDATE data_plugin_instances SET last_fetch_at = CURRENT_TIMESTAMP, last_error = ? WHERE id = ?`,
		fetchErr.Error(), instanceID,
	)
	if err != nil {
		return fmt.Errorf("recording fetch error for instance %d: %w", instanceID, err)
	}
	return nil
}

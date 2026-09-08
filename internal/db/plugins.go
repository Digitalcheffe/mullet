package db

import (
	"database/sql"
	"fmt"
	"time"
)

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

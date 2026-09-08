package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

type pluginStatusResponse struct {
	ID             int     `json:"id"`
	PluginID       string  `json:"plugin_id"`
	InstanceName   string  `json:"instance_name"`
	RefreshSeconds int     `json:"refresh_seconds"`
	Enabled        bool    `json:"enabled"`
	Status         string  `json:"status"` // "synced" | "retrying" | "pending" | "disabled"
	LastError      *string `json:"last_error,omitempty"`
}

type dashboardResponse struct {
	UptimeSeconds     int64                  `json:"uptime_seconds"`
	PluginCount       int                    `json:"plugin_count"`
	DisplayCount      int                    `json:"display_count"`
	ActiveClientCount int                    `json:"active_client_count"`
	SystemStatus      string                 `json:"system_status"` // "normal" | "attention"
	Plugins           []pluginStatusResponse `json:"plugins"`
}

// handleDashboard reports the system overview shown on the admin
// dashboard.
func handleDashboard(sqldb *sql.DB, info ServerInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses, err := db.ListPluginInstanceStatuses(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		plugins := make([]pluginStatusResponse, 0, len(statuses))
		systemStatus := "normal"
		for _, s := range statuses {
			status := pluginStatus(s)
			if status == "retrying" {
				systemStatus = "attention"
			}

			plugins = append(plugins, pluginStatusResponse{
				ID:             s.ID,
				PluginID:       s.PluginID,
				InstanceName:   s.InstanceName,
				RefreshSeconds: int(s.RefreshInterval.Seconds()),
				Enabled:        s.Enabled,
				Status:         status,
				LastError:      s.LastError,
			})
		}

		displays, err := db.ListDisplays(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		clients, err := db.ListClients(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		activeClients := 0
		for _, c := range clients {
			if isClientOnline(c) {
				activeClients++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dashboardResponse{
			UptimeSeconds:     int64(time.Since(info.StartedAt).Seconds()),
			PluginCount:       len(statuses),
			DisplayCount:      len(displays),
			ActiveClientCount: activeClients,
			SystemStatus:      systemStatus,
			Plugins:           plugins,
		})
	}
}

// pluginStatus derives a display-ready status string ("synced" |
// "retrying" | "pending" | "disabled") from a plugin instance's fetch
// state. Shared by the dashboard's Plugin Status panel and the plugin
// management page's instance list.
func pluginStatus(s db.PluginInstanceStatus) string {
	switch {
	case !s.Enabled:
		return "disabled"
	case s.LastError != nil:
		return "retrying"
	case s.LastFetchAt != nil:
		return "synced"
	default:
		return "pending"
	}
}

const serverNameSettingKey = "server_name"
const defaultServerName = "Mullet"

type settingsResponse struct {
	ServerName string `json:"server_name"`
	Port       string `json:"port"`
	DBPath     string `json:"db_path"`
}

// handleGetSettings returns the current system settings: the editable
// server name plus read-only bootstrap info (port, DB path) that can only
// change via env vars and a restart.
func handleGetSettings(sqldb *sql.DB, info ServerInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, found, err := db.GetSetting(sqldb, serverNameSettingKey)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !found {
			name = defaultServerName
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settingsResponse{ServerName: name, Port: info.Port, DBPath: info.DBPath})
	}
}

type updateSettingsRequest struct {
	ServerName string `json:"server_name"`
}

// handlePutSettings updates the editable system settings (currently just
// the server name).
func handlePutSettings(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ServerName == "" {
			http.Error(w, "server_name is required", http.StatusBadRequest)
			return
		}

		if err := db.SetSetting(sqldb, serverNameSettingKey, req.ServerName); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

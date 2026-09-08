package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
)

type selectOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type setupFieldResponse struct {
	Key         string                 `json:"key"`
	Label       string                 `json:"label"`
	Type        string                 `json:"type"`
	Required    bool                   `json:"required"`
	Default     any                    `json:"default,omitempty"`
	Placeholder string                 `json:"placeholder,omitempty"`
	HelpText    string                 `json:"help_text,omitempty"`
	Options     []selectOptionResponse `json:"options,omitempty"`
}

type pluginManifestResponse struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	DataShapes          []string             `json:"data_shapes"`
	SetupFields         []setupFieldResponse `json:"setup_fields"`
	AuthType            string               `json:"auth_type"`
	RecommendedInterval int                  `json:"recommended_interval_seconds"`
	MinInterval         int                  `json:"min_interval_seconds"`
}

func toManifestResponse(m plugindata.DataPluginManifest) pluginManifestResponse {
	fields := make([]setupFieldResponse, len(m.SetupFields))
	for i, f := range m.SetupFields {
		options := make([]selectOptionResponse, len(f.Options))
		for j, o := range f.Options {
			options[j] = selectOptionResponse{Value: o.Value, Label: o.Label}
		}
		fields[i] = setupFieldResponse{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required,
			Default: f.Default, Placeholder: f.Placeholder, HelpText: f.HelpText, Options: options,
		}
	}
	return pluginManifestResponse{
		ID:                  m.ID,
		Name:                m.Name,
		Description:         m.Description,
		DataShapes:          m.DataShapes,
		SetupFields:         fields,
		AuthType:            m.AuthType,
		RecommendedInterval: int(m.RecommendedInterval.Seconds()),
		MinInterval:         int(m.MinInterval.Seconds()),
	}
}

// handleListPlugins returns every registered plugin type's manifest, for
// the "available plugins" list an admin picks from to add an instance.
func handleListPlugins(registry *plugindata.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plugins := registry.List()
		resp := make([]pluginManifestResponse, len(plugins))
		for i, p := range plugins {
			resp[i] = toManifestResponse(p.Manifest())
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type pluginInstanceResponse struct {
	ID             int             `json:"id"`
	PluginID       string          `json:"plugin_id"`
	InstanceName   string          `json:"instance_name"`
	Config         json.RawMessage `json:"config"`
	RefreshSeconds int             `json:"refresh_seconds"`
	Enabled        bool            `json:"enabled"`
	Status         string          `json:"status"`
	LastError      *string         `json:"last_error,omitempty"`
}

func toInstanceResponse(s db.PluginInstanceStatus) pluginInstanceResponse {
	config := s.Config
	if config == "" {
		config = "{}"
	}
	return pluginInstanceResponse{
		ID:             s.ID,
		PluginID:       s.PluginID,
		InstanceName:   s.InstanceName,
		Config:         json.RawMessage(config),
		RefreshSeconds: int(s.RefreshInterval.Seconds()),
		Enabled:        s.Enabled,
		Status:         pluginStatus(s),
		LastError:      s.LastError,
	}
}

// handleListPluginInstances lists every configured plugin instance.
func handleListPluginInstances(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses, err := db.ListPluginInstanceStatuses(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]pluginInstanceResponse, len(statuses))
		for i, s := range statuses {
			resp[i] = toInstanceResponse(s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type instanceRequest struct {
	PluginID       string          `json:"plugin_id"`
	InstanceName   string          `json:"instance_name"`
	RefreshSeconds int             `json:"refresh_seconds"`
	Enabled        bool            `json:"enabled"`
	Config         json.RawMessage `json:"config"`
}

// handleCreatePluginInstance creates a new plugin instance, validating
// its config against the plugin's manifest, and reloads the scheduler so
// it starts running immediately -- no restart needed.
func handleCreatePluginInstance(sqldb *sql.DB, registry *plugindata.Registry, sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req instanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.InstanceName == "" {
			http.Error(w, "instance_name is required", http.StatusBadRequest)
			return
		}

		plugin, ok := registry.Get(req.PluginID)
		if !ok {
			http.Error(w, fmt.Sprintf("unknown plugin_id %q", req.PluginID), http.StatusBadRequest)
			return
		}

		configJSON, configMap, err := parseInstanceConfig(req.Config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateSetupFields(plugin.Manifest(), configMap); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		refreshSeconds := req.RefreshSeconds
		if refreshSeconds <= 0 {
			refreshSeconds = int(plugin.Manifest().RecommendedInterval.Seconds())
		}

		id, err := db.CreatePluginInstance(sqldb, req.PluginID, req.InstanceName, refreshSeconds, req.Enabled, configJSON)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := sched.Reload(); err != nil {
			log.Printf("plugin instance %d created but scheduler reload failed: %v", id, err)
		}

		writeInstance(w, sqldb, id, http.StatusCreated)
	}
}

// handleUpdatePluginInstance updates an existing plugin instance and
// reloads the scheduler so the change (including enable/disable) takes
// effect immediately.
func handleUpdatePluginInstance(sqldb *sql.DB, registry *plugindata.Registry, sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		existing, err := db.GetPluginInstance(sqldb, id)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var req instanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.InstanceName == "" {
			http.Error(w, "instance_name is required", http.StatusBadRequest)
			return
		}

		plugin, ok := registry.Get(existing.PluginID)
		if !ok {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		configJSON, configMap, err := parseInstanceConfig(req.Config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateSetupFields(plugin.Manifest(), configMap); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		refreshSeconds := req.RefreshSeconds
		if refreshSeconds <= 0 {
			refreshSeconds = int(plugin.Manifest().RecommendedInterval.Seconds())
		}

		if err := db.UpdatePluginInstance(sqldb, id, req.InstanceName, refreshSeconds, req.Enabled, configJSON); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := sched.Reload(); err != nil {
			log.Printf("plugin instance %d updated but scheduler reload failed: %v", id, err)
		}

		writeInstance(w, sqldb, id, http.StatusOK)
	}
}

// handleDeletePluginInstance removes a plugin instance and reloads the
// scheduler so it stops immediately.
func handleDeletePluginInstance(sqldb *sql.DB, sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		if err := db.DeletePluginInstance(sqldb, id); errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := sched.Reload(); err != nil {
			log.Printf("plugin instance %d deleted but scheduler reload failed: %v", id, err)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func writeInstance(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	inst, err := db.GetPluginInstance(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toInstanceResponse(inst))
}

// parseInstanceConfig decodes a submitted config (a JSON object, or
// absent) into both the raw string stored in the DB and a map for
// manifest validation.
func parseInstanceConfig(raw json.RawMessage) (jsonStr string, configMap map[string]any, err error) {
	configMap = map[string]any{}
	if len(raw) == 0 {
		return "{}", configMap, nil
	}
	if err := json.Unmarshal(raw, &configMap); err != nil {
		return "", nil, fmt.Errorf("config must be a JSON object")
	}
	return string(raw), configMap, nil
}

// validateSetupFields checks config against a plugin's manifest: required
// fields must be present and non-empty, and typed fields (select,
// multi-select, toggle, number) must hold a value of the right shape.
func validateSetupFields(manifest plugindata.DataPluginManifest, config map[string]any) error {
	for _, field := range manifest.SetupFields {
		val, present := config[field.Key]
		if field.Required && (!present || isEmptyString(val)) {
			return fmt.Errorf("%s is required", field.Label)
		}
		if !present || val == nil {
			continue
		}

		switch field.Type {
		case "select":
			s, ok := val.(string)
			if !ok || !isValidOption(field.Options, s) {
				return fmt.Errorf("%s: invalid value", field.Label)
			}
		case "multi-select":
			arr, ok := val.([]any)
			if !ok {
				return fmt.Errorf("%s must be a list", field.Label)
			}
			for _, v := range arr {
				s, ok := v.(string)
				if !ok || !isValidOption(field.Options, s) {
					return fmt.Errorf("%s: invalid value %v", field.Label, v)
				}
			}
		case "toggle":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("%s must be true or false", field.Label)
			}
		case "number":
			if _, ok := val.(float64); !ok {
				return fmt.Errorf("%s must be a number", field.Label)
			}
		}
	}
	return nil
}

func isEmptyString(v any) bool {
	s, ok := v.(string)
	return ok && s == ""
}

func isValidOption(options []plugindata.SelectOption, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

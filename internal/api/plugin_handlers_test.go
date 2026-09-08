package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func TestListPlugins(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var manifests []pluginManifestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &manifests); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("manifests = %+v, want 2 entries (clock, openweathermap)", manifests)
	}

	byID := map[string]pluginManifestResponse{}
	for _, m := range manifests {
		byID[m.ID] = m
	}
	if byID["clock"].AuthType != "none" {
		t.Errorf("clock AuthType = %q, want none", byID["clock"].AuthType)
	}
	if byID["openweathermap"].AuthType != "api_key" {
		t.Errorf("openweathermap AuthType = %q, want api_key", byID["openweathermap"].AuthType)
	}
	if len(byID["openweathermap"].SetupFields) == 0 {
		t.Error("openweathermap has no SetupFields, want api_key/location/units")
	}
}

func TestCreatePluginInstance(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded instances: %v", err)
	}

	body, _ := json.Marshal(instanceRequest{
		PluginID:     "clock",
		InstanceName: "Kitchen Clock",
		Enabled:      true,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created pluginInstanceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if created.PluginID != "clock" || created.InstanceName != "Kitchen Clock" || !created.Enabled {
		t.Errorf("created = %+v, unexpected values", created)
	}
	if created.RefreshSeconds != 60 {
		t.Errorf("RefreshSeconds = %d, want 60 (clock's recommended interval, since none was given)", created.RefreshSeconds)
	}

	// Confirm it actually landed in the list too.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances", nil))
	var list []pluginInstanceResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("instance list has %d entries, want 1", len(list))
	}
}

func TestCreatePluginInstanceUnknownPluginID(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(instanceRequest{PluginID: "does-not-exist", InstanceName: "X"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreatePluginInstanceRejectsEmptyName(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(instanceRequest{PluginID: "clock", InstanceName: ""})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty instance_name: status = %d, want 400", rec.Code)
	}
}

func TestCreatePluginInstanceValidatesRequiredConfig(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	// openweathermap requires api_key and location; submitting neither
	// should fail manifest validation rather than silently create a
	// broken instance.
	body, _ := json.Marshal(instanceRequest{
		PluginID:     "openweathermap",
		InstanceName: "Home",
		Config:       json.RawMessage(`{}`),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing required config: status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// With both required fields present, it should succeed. Disabled, so
	// creating it doesn't trigger a live fetch against the real API via
	// the scheduler's Reload() -- this test is about validation, not
	// fetching.
	body, _ = json.Marshal(instanceRequest{
		PluginID:     "openweathermap",
		InstanceName: "Home",
		Enabled:      false,
		Config:       json.RawMessage(`{"api_key":"k","location":"Seattle","units":"metric"}`),
	})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid config: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreatePluginInstanceRejectsInvalidSelectValue(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(instanceRequest{
		PluginID:     "openweathermap",
		InstanceName: "Home",
		Config:       json.RawMessage(`{"api_key":"k","location":"Seattle","units":"bogus"}`),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid select value: status = %d, want 400", rec.Code)
	}
}

func TestUpdatePluginInstance(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded instances: %v", err)
	}
	id, err := insertTestInstance(sqldb, "clock", "Kitchen Clock", 60, true, "{}")
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}

	body, _ := json.Marshal(instanceRequest{
		PluginID:       "clock",
		InstanceName:   "Office Clock",
		RefreshSeconds: 120,
		Enabled:        false,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/plugins/instances/"+strconv.Itoa(id), body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated pluginInstanceResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.InstanceName != "Office Clock" || updated.Enabled || updated.RefreshSeconds != 120 {
		t.Errorf("updated = %+v, unexpected values", updated)
	}
}

func TestUpdatePluginInstanceNotFound(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(instanceRequest{PluginID: "clock", InstanceName: "X"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/plugins/instances/9999", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeletePluginInstance(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	id, err := insertTestInstance(sqldb, "clock", "Kitchen Clock", 60, true, "{}")
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/plugins/instances/"+strconv.Itoa(id), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	var count int
	sqldb.QueryRow(`SELECT COUNT(*) FROM data_plugin_instances WHERE id = ?`, id).Scan(&count)
	if count != 0 {
		t.Errorf("instance %d still exists after delete", id)
	}
}

func TestDeletePluginInstanceNotFound(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/plugins/instances/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestTestPluginInstanceSuccess(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	id, err := insertTestInstance(sqldb, "clock", "Kitchen Clock", 60, true, "{}")
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances/"+strconv.Itoa(id)+"/test", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var result testInstanceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !result.Success || result.Error != "" {
		t.Errorf("result = %+v, want success with no error", result)
	}
}

func TestTestPluginInstanceNotFound(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances/9999/test", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPluginEndpointsRequireAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/admin/plugins", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/plugins/instances", nil),
		httptest.NewRequest(http.MethodPost, "/api/admin/plugins/instances", bytes.NewReader([]byte(`{}`))),
		httptest.NewRequest(http.MethodPost, "/api/admin/plugins/instances/1/test", nil),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func insertTestInstance(sqldb *sql.DB, pluginID, name string, refreshSeconds int, enabled bool, config string) (int, error) {
	return db.CreatePluginInstance(sqldb, pluginID, name, refreshSeconds, enabled, config)
}

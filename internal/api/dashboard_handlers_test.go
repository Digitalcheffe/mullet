package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/auth"
)

func authedRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	token, err := auth.IssueToken([]byte(testJWTSecret), 1, "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestDashboard(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)

	// Migrations seed a clock plugin instance for a real deployment; clear
	// it so the count below is deterministic.
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (plugin_id, instance_name, refresh_seconds) VALUES ('clock', 'Kitchen Clock', 60)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/dashboard", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding dashboard response: %v", err)
	}
	if resp.PluginCount != 1 {
		t.Errorf("PluginCount = %d, want 1", resp.PluginCount)
	}
	if resp.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, want >= 0", resp.UptimeSeconds)
	}
}

func TestGetSettingsDefaultsServerName(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings", nil))

	var resp settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding settings response: %v", err)
	}
	if resp.ServerName != defaultServerName {
		t.Errorf("ServerName = %q, want default %q", resp.ServerName, defaultServerName)
	}
	if resp.Port != "8080" || resp.DBPath != "/data/mullet.db" {
		t.Errorf("Port/DBPath = %q/%q, want 8080//data/mullet.db", resp.Port, resp.DBPath)
	}
}

func TestPutSettingsUpdatesServerName(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(map[string]string{"server_name": "Kitchen Server"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings", nil))
	var resp settingsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ServerName != "Kitchen Server" {
		t.Errorf("ServerName after update = %q, want %q", resp.ServerName, "Kitchen Server")
	}
}

func TestPutSettingsRejectsEmptyName(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(map[string]string{"server_name": ""})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDashboardAndSettingsRequireAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	for _, path := range []string{"/api/admin/dashboard", "/api/admin/settings"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without token: status = %d, want 401", path, rec.Code)
		}
	}
}

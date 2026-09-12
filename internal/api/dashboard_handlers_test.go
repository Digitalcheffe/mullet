package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/logging"
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
	if len(resp.Plugins) != 1 {
		t.Fatalf("Plugins has %d entries, want 1", len(resp.Plugins))
	}
	if resp.Plugins[0].Status != "pending" {
		t.Errorf("Plugins[0].Status = %q, want pending (never fetched)", resp.Plugins[0].Status)
	}
	if resp.SystemStatus != "normal" {
		t.Errorf("SystemStatus = %q, want normal", resp.SystemStatus)
	}
}

func TestDashboardDisplaySummaries(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	displayID := createTestDisplay(t, router)

	screenBody, _ := json.Marshal(screenRequest{Name: "Main"})
	screenRec := httptest.NewRecorder()
	router.ServeHTTP(screenRec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", screenBody))
	if screenRec.Code != http.StatusCreated {
		t.Fatalf("create screen: status = %d, want 201 (body: %s)", screenRec.Code, screenRec.Body.String())
	}
	var screen screenResponse
	json.Unmarshal(screenRec.Body.Bytes(), &screen)
	screenID := screen.ID

	c := registerTestClient(t, router, "x7k-m2p")
	approveBody, _ := json.Marshal(approveClientRequest{DisplayID: displayID})
	router.ServeHTTP(httptest.NewRecorder(), authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", approveBody))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/config", nil)) // poll once so last_seen_at is recent

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/dashboard", nil))
	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding dashboard response: %v", err)
	}

	var found *dashboardDisplayResponse
	for i := range resp.Displays {
		if resp.Displays[i].ID == displayID {
			found = &resp.Displays[i]
		}
	}
	if found == nil {
		t.Fatalf("Displays = %+v, missing display %d", resp.Displays, displayID)
	}
	if found.ScreenCount != 1 || found.FirstScreenID == nil || *found.FirstScreenID != screenID {
		t.Errorf("display summary = %+v, want ScreenCount=1 FirstScreenID=%d", found, screenID)
	}
	if !found.Online {
		t.Error("display summary Online = false, want true (an approved, recently-polled client is assigned to it)")
	}
}

func TestDashboardPluginStatuses(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, enabled) VALUES
			(1, 'openweathermap', 'Home', 900, 1),
			(2, 'clock', 'Office Clock', 60, 0)`,
	); err != nil {
		t.Fatalf("seeding plugin instances: %v", err)
	}
	if _, err := sqldb.Exec(
		`UPDATE data_plugin_instances SET last_error = 'invalid API key', last_fetch_at = CURRENT_TIMESTAMP WHERE id = 1`,
	); err != nil {
		t.Fatalf("seeding error state: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/dashboard", nil))

	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding dashboard response: %v", err)
	}

	if resp.SystemStatus != "attention" {
		t.Errorf("SystemStatus = %q, want attention (a plugin is retrying)", resp.SystemStatus)
	}

	byID := map[int]pluginStatusResponse{}
	for _, p := range resp.Plugins {
		byID[p.ID] = p
	}
	if byID[1].Status != "retrying" {
		t.Errorf("plugin 1 status = %q, want retrying", byID[1].Status)
	}
	if byID[2].Status != "disabled" {
		t.Errorf("plugin 2 status = %q, want disabled", byID[2].Status)
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

// resetLoggingToStdout undoes a test's own logging.Configure call --
// handlePutSettings changes process-wide log output for real, and every
// other test in this package (middleware's own access-log line alone)
// calls log.Printf too, so a test that points logging at a temp file
// must always put it back before that directory is removed.
func resetLoggingToStdout(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if err := logging.Configure(""); err != nil {
			t.Errorf("resetting logging to stdout: %v", err)
		}
	})
}

func TestGetSettingsDefaultsLogFilePathToEmpty(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings", nil))
	var resp settingsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.LogFilePath != "" {
		t.Errorf("LogFilePath (unsaved) = %q, want empty (stdout only)", resp.LogFilePath)
	}
}

func TestPutSettingsSavesLogFilePathAndAppliesIt(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	path := filepath.Join(t.TempDir(), "mullet.log")
	resetLoggingToStdout(t) // registered after TempDir's own cleanup (Windows can't delete an open file)

	body, _ := json.Marshal(updateSettingsRequest{ServerName: "Mullet", LogFilePath: path})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings", nil))
	var resp settingsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.LogFilePath != path {
		t.Errorf("LogFilePath after update = %q, want %q", resp.LogFilePath, path)
	}

	// The setting isn't just persisted -- it took effect immediately,
	// without a restart.
	log.Print("PUT_SETTINGS_LOG_TEST")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "PUT_SETTINGS_LOG_TEST") {
		t.Errorf("log file = %q, want it to contain the logged message", data)
	}
}

func TestPutSettingsRejectsUnwritableLogFilePath(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	// A path that's actually an existing directory can never be opened
	// as a log file -- logging.Configure creates a merely-missing
	// parent directory rather than rejecting it, so that alone no
	// longer exercises this rejection path (see logging_test.go's
	// TestConfigureCreatesParentDirectory).
	tmp := t.TempDir()
	badPath := filepath.Join(tmp, "not-a-file")
	if err := os.Mkdir(badPath, 0o755); err != nil {
		t.Fatalf("creating directory at badPath: %v", err)
	}
	resetLoggingToStdout(t)

	body, _ := json.Marshal(updateSettingsRequest{ServerName: "Mullet", LogFilePath: badPath})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// Rejected outright, not silently saved with a broken destination.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings", nil))
	var resp settingsResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.LogFilePath != "" {
		t.Errorf("LogFilePath after a rejected update = %q, want still empty", resp.LogFilePath)
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

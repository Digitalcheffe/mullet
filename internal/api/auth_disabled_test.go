package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newAuthDisabledTestRouter(t *testing.T) http.Handler {
	t.Helper()
	sqldb := newTestUserDB(t)
	registry, sched := newTestSchedulerDeps(t, sqldb)
	return NewRouter(sqldb, []byte(testJWTSecret), nil, testServerInfo(), t.TempDir(), true, registry, sched, t.TempDir())
}

func TestAuthDisabledSetupStatusReportsNotRequired(t *testing.T) {
	router := newAuthDisabledTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/setup", nil))

	var status setupStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decoding setup status: %v", err)
	}
	if status.Required {
		t.Error("Required = true with AuthDisabled, want false")
	}
	if !status.AuthDisabled {
		t.Error("AuthDisabled = false, want true")
	}
}

func TestAuthDisabledAllowsAdminRoutesWithoutToken(t *testing.T) {
	router := newAuthDisabledTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/me", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var who map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &who); err != nil {
		t.Fatalf("decoding whoami response: %v", err)
	}
	if who["username"] != "dev" {
		t.Errorf("username = %v, want dev", who["username"])
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/api/admin/dashboard without token: status = %d, want 200", rec.Code)
	}
}

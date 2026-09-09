package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	clockplugin "github.com/Digitalcheffe/mullet/internal/plugins/data/clock"
	owmplugin "github.com/Digitalcheffe/mullet/internal/plugins/data/openweathermap"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
)

const testJWTSecret = "test-secret"

func testServerInfo() ServerInfo {
	return ServerInfo{Port: "8080", DBPath: "/data/mullet.db", StartedAt: time.Now()}
}

// newTestUserDB returns a migrated database seeded with one admin user
// (username "admin", password "s3cret").
func newTestUserDB(t *testing.T) *sql.DB {
	t.Helper()

	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO users (username, password_hash) VALUES ('admin', ?)`, hash,
	); err != nil {
		t.Fatalf("seeding user: %v", err)
	}

	return sqldb
}

// newTestSchedulerDeps returns a registry seeded with two real plugins --
// clock (no setup fields, for the simple paths) and openweathermap (has
// required setup fields, for manifest-validation tests) -- plus a started
// scheduler bound to sqldb. Both are suitable for passing into NewRouter.
func newTestSchedulerDeps(t *testing.T, sqldb *sql.DB) (*plugindata.Registry, *scheduler.Scheduler) {
	t.Helper()

	registry := plugindata.NewRegistry()
	if err := registry.Register(clockplugin.New()); err != nil {
		t.Fatalf("registering clock plugin: %v", err)
	}
	if err := registry.Register(owmplugin.New()); err != nil {
		t.Fatalf("registering openweathermap plugin: %v", err)
	}

	sched := scheduler.New(sqldb, registry)
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("starting scheduler: %v", err)
	}
	t.Cleanup(sched.Stop)

	return registry, sched
}

func newTestRouter(t *testing.T, corsOrigins []string) (http.Handler, *sql.DB) {
	t.Helper()
	sqldb := newTestUserDB(t)
	registry, sched := newTestSchedulerDeps(t, sqldb)
	router := NewRouter(sqldb, []byte(testJWTSecret), corsOrigins, testServerInfo(), t.TempDir(), false, registry, sched, t.TempDir())
	return router, sqldb
}

func TestHealthz(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginSuccessAndFailure(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding login response: %v", err)
	}
	if resp.Token == "" {
		t.Error("login response has empty token")
	}

	badBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	req = httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(badBody))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login with wrong password status = %d, want 401", rec.Code)
	}
}

func TestAdminRouteRequiresAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage token: status = %d, want 401", rec.Code)
	}

	token, err := auth.IssueToken([]byte(testJWTSecret), 1, "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var who map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &who); err != nil {
		t.Fatalf("decoding whoami response: %v", err)
	}
	if who["username"] != "admin" {
		t.Errorf("username = %v, want admin", who["username"])
	}
}

func TestLoginEndToEndTokenGrantsAccess(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body)))

	var resp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+resp.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status with login-issued token = %d, want 200", rec.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	router, _ := newTestRouter(t, []string{"https://admin.example.com"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want allowed origin echoed back", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q for disallowed origin, want empty", got)
	}

	req = httptest.NewRequest(http.MethodOptions, "/api/admin/login", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS preflight status = %d, want 204", rec.Code)
	}
}

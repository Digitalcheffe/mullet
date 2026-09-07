package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
)

const testJWTSecret = "test-secret"

func newTestRouter(t *testing.T, corsOrigins []string) (http.Handler, *sql.DB) {
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

	router := NewRouter(sqldb, []byte(testJWTSecret), corsOrigins)
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

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// newUnseededTestRouter is like newTestRouter but doesn't create a user,
// so setup is still required.
func newUnseededTestRouter(t *testing.T) http.Handler {
	t.Helper()

	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return NewRouter(sqldb, []byte(testJWTSecret), nil, testServerInfo(), t.TempDir())
}

func TestSetupStatus(t *testing.T) {
	router := newUnseededTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/setup", nil))

	var status setupStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decoding setup status: %v", err)
	}
	if !status.Required {
		t.Error("Required = false on a fresh database, want true")
	}
}

func TestSetupCreatesAdminAndIssuesToken(t *testing.T) {
	router := newUnseededTestRouter(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret123"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/setup", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding setup response: %v", err)
	}
	if resp.Token == "" {
		t.Error("setup response has empty token")
	}

	// Setup status now reports not required.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/setup", nil))
	var status setupStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &status)
	if status.Required {
		t.Error("Required = true after setup completed, want false")
	}

	// The issued token grants access to protected routes.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+resp.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/api/admin/me with setup-issued token = %d, want 200", rec.Code)
	}
}

func TestSetupRejectsSecondCall(t *testing.T) {
	router := newUnseededTestRouter(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret123"})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/admin/setup", bytes.NewReader(body)))

	body2, _ := json.Marshal(map[string]string{"username": "second-admin", "password": "s3cret123"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/setup", bytes.NewReader(body2)))

	if rec.Code != http.StatusConflict {
		t.Errorf("second setup call status = %d, want 409", rec.Code)
	}
}

func TestSetupRejectsShortPassword(t *testing.T) {
	router := newUnseededTestRouter(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "short"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/setup", bytes.NewReader(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("short password status = %d, want 400", rec.Code)
	}
}

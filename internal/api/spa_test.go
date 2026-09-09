package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSPAHandlerServesStaticFileAndFallsBackToIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatalf("writing index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("writing app.js: %v", err)
	}

	handler := newSPAHandler(dir)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Errorf("static file: status=%d body=%q, want 200 %q", rec.Code, rec.Body.String(), "console.log(1)")
	}

	for _, path := range []string{"/admin", "/admin/settings", "/display/kitchen", "/"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "<html>shell</html>" {
			t.Errorf("%s: status=%d body=%q, want 200 index.html fallback", path, rec.Code, rec.Body.String())
		}
	}
}

func TestRouterServesFrontendWithoutShadowingAPI(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatalf("writing index.html: %v", err)
	}

	sqldb := newTestUserDB(t)
	registry, sched := newTestSchedulerDeps(t, sqldb)
	router := NewRouter(sqldb, []byte(testJWTSecret), nil, testServerInfo(), staticDir, false, registry, sched, t.TempDir())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/settings", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>shell</html>" {
		t.Errorf("/admin/settings: status=%d body=%q, want SPA shell", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("/healthz still serves its own handler: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/admin/me without token: status=%d, want 401 (not shadowed by SPA fallback)", rec.Code)
	}
}

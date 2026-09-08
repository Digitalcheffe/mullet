package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	clockplugin "github.com/Digitalcheffe/mullet/internal/plugins/data/clock"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
)

// fakeOAuthPlugin is a minimal DataPlugin whose manifest declares
// AuthType "oauth2", used only to exercise the generic OAuth2 handler
// end-to-end (BuildAuthURL / Exchange / token storage) without needing a
// real Microsoft Graph plugin (#25) or real Microsoft credentials.
type fakeOAuthPlugin struct {
	tokenURL string
}

func (p *fakeOAuthPlugin) ID() string   { return "test-oauth" }
func (p *fakeOAuthPlugin) Name() string { return "Test OAuth Plugin" }
func (p *fakeOAuthPlugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:       "test-oauth",
		Name:     "Test OAuth Plugin",
		AuthType: "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{
			AuthURL:     p.tokenURL + "/authorize/{tenant}",
			TokenURL:    p.tokenURL + "/token",
			Scopes:      []string{"Calendars.Read", "offline_access"},
			TenantField: "tenant",
		},
		SetupFields: []plugindata.SetupField{
			{Key: "client_id", Label: "Client ID", Type: "text"},
			{Key: "client_secret", Label: "Client Secret", Type: "password"},
			{Key: "tenant", Label: "Tenant", Type: "text"},
		},
	}
}
func (p *fakeOAuthPlugin) DataShapes() []string               { return nil }
func (p *fakeOAuthPlugin) RefreshInterval() time.Duration     { return time.Minute }
func (p *fakeOAuthPlugin) Configure(cfg map[string]any) error { return nil }
func (p *fakeOAuthPlugin) Fetch(ctx context.Context) (map[string][]any, error) {
	return map[string][]any{}, nil
}

// fakeProviderServer mimics a real OAuth2 provider's token endpoint
// (matching the shape of internal/oauth's own test fixture) at a fixed
// URL this test controls, so BuildAuthURL/Exchange can be exercised for
// real over HTTP without any external dependency.
func fakeProviderServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing token request form: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("code") != "auth-code-1" {
			http.Error(w, "unexpected code", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-from-code", "refresh_token": "refresh-1",
			"token_type": "Bearer", "expires_in": 3600,
		})
	}))
}

// newTestRouterWithOAuthPlugin is like newTestRouter but its registry
// also has fakeOAuthPlugin registered, pointed at a fake provider's
// tokenURL, for tests that exercise the OAuth2 flow end-to-end.
func newTestRouterWithOAuthPlugin(t *testing.T, tokenURL string) (http.Handler, *sql.DB) {
	t.Helper()
	sqldb := newTestUserDB(t)

	registry := plugindata.NewRegistry()
	if err := registry.Register(clockplugin.New()); err != nil {
		t.Fatalf("registering clock plugin: %v", err)
	}
	if err := registry.Register(&fakeOAuthPlugin{tokenURL: tokenURL}); err != nil {
		t.Fatalf("registering fake oauth plugin: %v", err)
	}

	sched := scheduler.New(sqldb, registry)
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("starting scheduler: %v", err)
	}
	t.Cleanup(sched.Stop)

	router := NewRouter(sqldb, []byte(testJWTSecret), nil, testServerInfo(), t.TempDir(), false, registry, sched)
	return router, sqldb
}

func createTestOAuthInstance(t *testing.T, router http.Handler, config string) int {
	t.Helper()
	body, _ := json.Marshal(instanceRequest{
		PluginID: "test-oauth", InstanceName: "Test Instance", RefreshSeconds: 300, Enabled: true,
		Config: json.RawMessage(config),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestOAuthInstance: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created pluginInstanceResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

// authorizeURL calls the authorize endpoint (as the frontend's
// authenticated fetch would) and returns the authorize_url from its
// JSON body -- see handleOAuthAuthorize's doc comment for why this
// endpoint returns JSON instead of redirecting itself.
func authorizeURL(t *testing.T, router http.Handler, instanceID int) string {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/oauth/authorize", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp authorizeURLResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding authorize response: %v", err)
	}
	return resp.AuthorizeURL
}

func TestOAuthAuthorizeReturnsURLWithState(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	loc, err := url.Parse(authorizeURL(t, router, instanceID))
	if err != nil {
		t.Fatalf("parsing authorize_url: %v", err)
	}
	if loc.Path != "/authorize/consumers" {
		t.Errorf("path = %q, want /authorize/consumers (tenant substituted)", loc.Path)
	}
	q := loc.Query()
	if q.Get("client_id") != "abc" {
		t.Errorf("client_id = %q, want abc", q.Get("client_id"))
	}
	if q.Get("state") == "" {
		t.Error("state is empty")
	}
	if q.Get("redirect_uri") == "" || !containsSuffix(q.Get("redirect_uri"), "/api/oauth/callback") {
		t.Errorf("redirect_uri = %q, want it to end in /api/oauth/callback", q.Get("redirect_uri"))
	}
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestOAuthAuthorizeMissingCredentialsReturns400(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{}`)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/oauth/authorize", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestOAuthAuthorizeNonOAuthPluginReturns400(t *testing.T) {
	router, sqldb := newTestRouterWithOAuthPlugin(t, "http://unused.example")
	instanceID, err := insertTestInstance(sqldb, "clock", "Kitchen Clock", 60, true, "{}")
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/oauth/authorize", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestOAuthCallbackSuccessStoresToken(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, sqldb := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	loc, _ := url.Parse(authorizeURL(t, router, instanceID))
	state := loc.Query().Get("state")

	callbackURL := "/api/oauth/callback?code=auth-code-1&state=" + url.QueryEscape(state)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, callbackURL, nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302 (body: %s)", rec.Code, rec.Body.String())
	}
	if loc, _ := url.Parse(rec.Header().Get("Location")); loc.Query().Get("oauth") != "success" {
		t.Errorf("redirect = %q, want oauth=success", rec.Header().Get("Location"))
	}

	tok, err := db.GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if tok.AccessToken != "access-from-code" || tok.RefreshToken == nil || *tok.RefreshToken != "refresh-1" {
		t.Errorf("tok = %+v, unexpected values", tok)
	}
	if tok.Scopes != "Calendars.Read offline_access" {
		t.Errorf("Scopes = %q, want the manifest's requested scopes joined", tok.Scopes)
	}
}

func TestOAuthCallbackInvalidStateFails(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=auth-code-1&state=bogus", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("oauth") != "error" {
		t.Errorf("redirect = %q, want oauth=error for an unrecognized state", rec.Header().Get("Location"))
	}
}

func TestOAuthCallbackStateIsSingleUse(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)
	loc, _ := url.Parse(authorizeURL(t, router, instanceID))
	state := loc.Query().Get("state")
	callbackURL := "/api/oauth/callback?code=auth-code-1&state=" + url.QueryEscape(state)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, callbackURL, nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("first callback status = %d, want 302", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, callbackURL, nil))
	loc, _ = url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("oauth") != "error" {
		t.Error("replaying the same callback URL succeeded a second time, want it rejected")
	}
}

func TestOAuthCallbackProviderDeniedConsent(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/oauth/callback?error=access_denied", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("oauth") != "error" {
		t.Errorf("redirect = %q, want oauth=error", rec.Header().Get("Location"))
	}
}

func TestOAuthAuthorizedStatusInInstanceResponse(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, sqldb := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	findInstance := func() pluginInstanceResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances", nil))
		var list []pluginInstanceResponse
		json.Unmarshal(rec.Body.Bytes(), &list)
		for _, inst := range list {
			if inst.ID == instanceID {
				return inst
			}
		}
		t.Fatalf("instance %d not found in list %+v", instanceID, list)
		return pluginInstanceResponse{}
	}

	if got := findInstance(); got.OAuthAuthorized {
		t.Fatalf("before authorize: oauth_authorized = %v, want false", got.OAuthAuthorized)
	}

	refresh := "refresh-1"
	if err := db.UpsertOAuthToken(sqldb, instanceID, "access-1", &refresh, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	if got := findInstance(); !got.OAuthAuthorized {
		t.Fatalf("after authorize: oauth_authorized = %v, want true", got.OAuthAuthorized)
	}
}

func TestNonOAuthPluginInstanceAlwaysReportsUnauthorized(t *testing.T) {
	router, _ := newTestRouterWithOAuthPlugin(t, "http://unused.example")
	body, _ := json.Marshal(instanceRequest{PluginID: "clock", InstanceName: "Kitchen Clock", RefreshSeconds: 60, Enabled: true})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	var created pluginInstanceResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.OAuthAuthorized {
		t.Error("a non-OAuth2 plugin instance reported oauth_authorized=true")
	}
}

func TestDeauthorizePluginInstance(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, sqldb := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)
	if err := db.UpsertOAuthToken(sqldb, instanceID, "access-1", nil, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/oauth", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	if _, err := db.GetOAuthToken(sqldb, instanceID); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("GetOAuthToken after deauthorize = %v, want ErrNotFound", err)
	}
}

func TestOAuthAdminEndpointsRequireAuth(t *testing.T) {
	router, _ := newTestRouterWithOAuthPlugin(t, "http://unused.example")

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/admin/plugins/instances/1/oauth/authorize", nil),
		httptest.NewRequest(http.MethodDelete, "/api/admin/plugins/instances/1/oauth", nil),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestOAuthCallbackDoesNotRequireAuth(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	// No Authorization header at all -- the callback must still be
	// reachable (it just rejects the bogus state), not 401.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=x&state=bogus", nil))
	if rec.Code == http.StatusUnauthorized {
		t.Error("callback returned 401 -- it must be reachable without a Bearer token")
	}
}

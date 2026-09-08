package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// Discover makes fakeOAuthPlugin satisfy plugindata.Discoverable,
// echoing the injected access token back into the option label so a
// test can confirm the token really was fetched/refreshed and passed
// through, not just that some canned response came back.
func (p *fakeOAuthPlugin) Discover(ctx context.Context, field string, cfg map[string]any) ([]plugindata.DiscoveredOption, error) {
	token, _ := cfg["access_token"].(string)
	return []plugindata.DiscoveredOption{
		{Value: "cal-1", Label: "Calendar One (token=" + token + ")"},
	}, nil
}

// fakeOAuthPluginRequiringToken behaves like a real OAuth2 plugin's
// Configure: it fails without an injected access_token, so a test can
// observe (via last_error clearing) whether the scheduler actually
// picked up a freshly authorized token promptly, rather than every
// fakeOAuthPlugin variant trivially succeeding regardless of cfg.
type fakeOAuthPluginRequiringToken struct {
	tokenURL string
}

func (p *fakeOAuthPluginRequiringToken) ID() string   { return "test-oauth-requires-token" }
func (p *fakeOAuthPluginRequiringToken) Name() string { return "Test OAuth Plugin (requires token)" }
func (p *fakeOAuthPluginRequiringToken) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID: "test-oauth-requires-token", Name: "Test OAuth Plugin (requires token)", AuthType: "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{AuthURL: p.tokenURL + "/authorize/{tenant}", TokenURL: p.tokenURL + "/token", TenantField: "tenant"},
		SetupFields: []plugindata.SetupField{
			{Key: "client_id", Label: "Client ID", Type: "text"},
			{Key: "client_secret", Label: "Client Secret", Type: "password"},
			{Key: "tenant", Label: "Tenant", Type: "text"},
		},
	}
}
func (p *fakeOAuthPluginRequiringToken) DataShapes() []string           { return nil }
func (p *fakeOAuthPluginRequiringToken) RefreshInterval() time.Duration { return time.Hour }
func (p *fakeOAuthPluginRequiringToken) Configure(cfg map[string]any) error {
	if cfg["access_token"] == nil || cfg["access_token"] == "" {
		return fmt.Errorf("access_token is required")
	}
	return nil
}
func (p *fakeOAuthPluginRequiringToken) Fetch(ctx context.Context) (map[string][]any, error) {
	return map[string][]any{}, nil
}

// fakeAPIKeyDiscoverablePlugin is a non-OAuth2 (AuthType "api_key")
// plugin that implements Discoverable -- for testing that discovery
// works for a plugin whose credentials are just a plain config field
// (home-assistant's static token, e.g.), with no separate authorize
// step at all, unlike every fakeOAuthPlugin variant above.
type fakeAPIKeyDiscoverablePlugin struct{}

func (p *fakeAPIKeyDiscoverablePlugin) ID() string   { return "test-apikey-discoverable" }
func (p *fakeAPIKeyDiscoverablePlugin) Name() string { return "Test API Key Plugin" }
func (p *fakeAPIKeyDiscoverablePlugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID: "test-apikey-discoverable", Name: "Test API Key Plugin", AuthType: "api_key",
		SetupFields: []plugindata.SetupField{
			{Key: "api_key", Label: "API Key", Type: "password"},
			{Key: "entities", Label: "Entities", Type: "multi-select", Dynamic: true},
		},
	}
}
func (p *fakeAPIKeyDiscoverablePlugin) DataShapes() []string               { return nil }
func (p *fakeAPIKeyDiscoverablePlugin) RefreshInterval() time.Duration     { return time.Minute }
func (p *fakeAPIKeyDiscoverablePlugin) Configure(cfg map[string]any) error { return nil }
func (p *fakeAPIKeyDiscoverablePlugin) Fetch(ctx context.Context) (map[string][]any, error) {
	return map[string][]any{}, nil
}
func (p *fakeAPIKeyDiscoverablePlugin) Discover(ctx context.Context, field string, cfg map[string]any) ([]plugindata.DiscoveredOption, error) {
	key, _ := cfg["api_key"].(string)
	return []plugindata.DiscoveredOption{{Value: "entity-1", Label: "Entity One (api_key=" + key + ")"}}, nil
}

// fakeOAuthPluginNoDiscover is an oauth2-manifest plugin that does NOT
// implement Discoverable, for testing that /discover fails
// gracefully against a plugin with no discovery support instead of
// panicking on the type assertion.
type fakeOAuthPluginNoDiscover struct {
	tokenURL string
}

func (p *fakeOAuthPluginNoDiscover) ID() string   { return "test-oauth-no-discover" }
func (p *fakeOAuthPluginNoDiscover) Name() string { return "Test OAuth Plugin (no discover)" }
func (p *fakeOAuthPluginNoDiscover) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID: "test-oauth-no-discover", Name: "Test OAuth Plugin (no discover)", AuthType: "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{AuthURL: p.tokenURL + "/authorize/{tenant}", TokenURL: p.tokenURL + "/token", TenantField: "tenant"},
		SetupFields: []plugindata.SetupField{
			{Key: "client_id", Label: "Client ID", Type: "text"},
			{Key: "client_secret", Label: "Client Secret", Type: "password"},
			{Key: "tenant", Label: "Tenant", Type: "text"},
		},
	}
}
func (p *fakeOAuthPluginNoDiscover) DataShapes() []string               { return nil }
func (p *fakeOAuthPluginNoDiscover) RefreshInterval() time.Duration     { return time.Minute }
func (p *fakeOAuthPluginNoDiscover) Configure(cfg map[string]any) error { return nil }
func (p *fakeOAuthPluginNoDiscover) Fetch(ctx context.Context) (map[string][]any, error) {
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
	if err := registry.Register(&fakeOAuthPluginNoDiscover{tokenURL: tokenURL}); err != nil {
		t.Fatalf("registering fake oauth plugin (no discover): %v", err)
	}
	if err := registry.Register(&fakeOAuthPluginRequiringToken{tokenURL: tokenURL}); err != nil {
		t.Fatalf("registering fake oauth plugin (requires token): %v", err)
	}
	if err := registry.Register(&fakeAPIKeyDiscoverablePlugin{}); err != nil {
		t.Fatalf("registering fake api_key discoverable plugin: %v", err)
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
	return createTestOAuthInstanceForPlugin(t, router, "test-oauth", config)
}

func createTestOAuthInstanceForPlugin(t *testing.T, router http.Handler, pluginID, config string) int {
	t.Helper()
	body, _ := json.Marshal(instanceRequest{
		PluginID: pluginID, InstanceName: "Test Instance", RefreshSeconds: 300, Enabled: true,
		Config: json.RawMessage(config),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/plugins/instances", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestOAuthInstanceForPlugin: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
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

// TestOAuthCallbackReloadsSchedulerImmediately guards a real bug found
// via live testing: without an explicit sched.Reload() after a
// successful callback, a freshly authorized instance kept showing its
// last tick's "not authorized yet" error until its next scheduled tick
// (up to a full refresh_seconds later) or an unrelated edit happened to
// trigger a reload -- every other plugin-instance mutation already
// reloads immediately, so authorizing should too.
func TestOAuthCallbackReloadsSchedulerImmediately(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstanceForPlugin(t, router, "test-oauth-requires-token", `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	findLastError := func() *string {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances", nil))
		var list []pluginInstanceResponse
		json.Unmarshal(rec.Body.Bytes(), &list)
		for _, inst := range list {
			if inst.ID == instanceID {
				return inst.LastError
			}
		}
		t.Fatalf("instance %d not found", instanceID)
		return nil
	}

	// Before authorizing, Configure fails for lack of a token -- the
	// scheduler's own initial Reload (on instance creation) already
	// records that.
	if err := findLastError(); err == nil {
		t.Fatal("last_error is nil before authorizing, want the missing-token error already recorded")
	}

	loc, _ := url.Parse(authorizeURL(t, router, instanceID))
	state := loc.Query().Get("state")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=auth-code-1&state="+url.QueryEscape(state), nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302 (body: %s)", rec.Code, rec.Body.String())
	}

	// No edit, no toggle, nothing else -- just the callback. The
	// scheduler's Reload restarts the instance's goroutine, which fetches
	// once immediately, so this should clear quickly without needing to
	// wait out a whole refresh interval (an hour, for this fake plugin).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := findLastError(); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("last_error never cleared after authorizing -- scheduler didn't reload promptly")
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

func TestOAuthDiscoverReturnsOptionsForAuthorizedInstance(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, sqldb := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)
	refresh := "refresh-1"
	if err := db.UpsertOAuthToken(sqldb, instanceID, "live-token", &refresh, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/discover?field=calendars", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var options []discoveredOptionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &options); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(options) != 1 || options[0].Value != "cal-1" {
		t.Fatalf("options = %+v, want one option with value cal-1", options)
	}
	if options[0].Label != "Calendar One (token=live-token)" {
		t.Errorf("label = %q, want the live token to have been passed through to Discover", options[0].Label)
	}
}

func TestOAuthDiscoverUnauthorizedInstanceReturns409(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/discover?field=calendars", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (not yet authorized)", rec.Code)
	}
}

func TestOAuthDiscoverMissingFieldParamReturns400(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)
	instanceID := createTestOAuthInstance(t, router, `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/discover", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestOAuthDiscoverPluginWithoutDiscoverableFails(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, sqldb := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstanceForPlugin(t, router, "test-oauth-no-discover", `{"client_id":"abc","client_secret":"xyz","tenant":"consumers"}`)
	if err := db.UpsertOAuthToken(sqldb, instanceID, "live-token", nil, time.Now().Add(time.Hour), ""); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/discover?field=calendars", nil))
	if rec.Code < 400 {
		t.Errorf("status = %d, want an error status for a plugin with no Discover support", rec.Code)
	}
}

// TestDiscoverWorksForNonOAuth2PluginWithoutAuthorizing guards handleDiscover's
// generalization beyond OAuth2 plugins: an api_key-type plugin (e.g.
// home-assistant, #27) has no separate "authorize" step at all -- its
// own config already has everything Discover needs the moment the
// instance exists, so discovery must work immediately, not require the
// oauth_authorized gate that only makes sense for AuthType "oauth2".
func TestDiscoverWorksForNonOAuth2PluginWithoutAuthorizing(t *testing.T) {
	srv := fakeProviderServer(t)
	defer srv.Close()
	router, _ := newTestRouterWithOAuthPlugin(t, srv.URL)

	instanceID := createTestOAuthInstanceForPlugin(t, router, "test-apikey-discoverable", `{"api_key":"secret-123"}`)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/plugins/instances/"+strconv.Itoa(instanceID)+"/discover?field=entities", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var options []discoveredOptionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &options); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(options) != 1 || options[0].Label != "Entity One (api_key=secret-123)" {
		t.Fatalf("options = %+v, want the plugin's own config value passed through", options)
	}
}

func TestOAuthAdminEndpointsRequireAuth(t *testing.T) {
	router, _ := newTestRouterWithOAuthPlugin(t, "http://unused.example")

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/admin/plugins/instances/1/oauth/authorize", nil),
		httptest.NewRequest(http.MethodDelete, "/api/admin/plugins/instances/1/oauth", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/plugins/instances/1/discover?field=calendars", nil),
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

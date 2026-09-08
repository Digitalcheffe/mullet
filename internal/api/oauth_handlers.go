package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/oauth"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

// loadOAuthPlugin loads instanceID's plugin instance and manifest,
// verifying the plugin is actually an OAuth2 one -- the shared first
// step of both the authorize and callback handlers, since both need to
// rebuild the identical oauth.Config (redirect URL included) that the
// authorization request was made with.
func loadOAuthPlugin(sqldb *sql.DB, registry *plugindata.Registry, instanceID int) (db.PluginInstanceStatus, plugindata.DataPluginManifest, error) {
	inst, err := db.GetPluginInstance(sqldb, instanceID)
	if err != nil {
		return db.PluginInstanceStatus{}, plugindata.DataPluginManifest{}, err
	}
	plugin, ok := registry.Get(inst.PluginID)
	if !ok {
		return db.PluginInstanceStatus{}, plugindata.DataPluginManifest{}, fmt.Errorf("unknown plugin_id %q", inst.PluginID)
	}
	manifest := plugin.Manifest()
	if manifest.AuthType != "oauth2" || manifest.OAuthConfig == nil {
		return db.PluginInstanceStatus{}, plugindata.DataPluginManifest{}, fmt.Errorf("plugin %q is not an OAuth2 plugin", inst.PluginID)
	}
	return inst, manifest, nil
}

// oauthConfigFor builds this instance's oauth.Config from its manifest
// (auth/token URLs, scopes, which field holds the tenant) and its own
// submitted config (client_id, client_secret, and the tenant value) --
// the two well-known keys every OAuth2 plugin's manifest is expected to
// declare as SetupFields, the same way openweathermap declares "api_key"
// for its own AuthType. redirectURL is derived per-request (see
// callbackURL) rather than configured, so it always matches the host the
// admin is actually browsing on.
func oauthConfigFor(manifest plugindata.DataPluginManifest, instanceConfig string, redirectURL string) (oauth.Config, error) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(instanceConfig), &cfg); err != nil {
		return oauth.Config{}, fmt.Errorf("instance config is not valid JSON: %w", err)
	}
	clientID, _ := cfg["client_id"].(string)
	clientSecret, _ := cfg["client_secret"].(string)
	if clientID == "" || clientSecret == "" {
		return oauth.Config{}, errors.New("client_id and client_secret must be configured before authorizing")
	}
	var tenant string
	if manifest.OAuthConfig.TenantField != "" {
		tenant, _ = cfg[manifest.OAuthConfig.TenantField].(string)
	}
	return oauth.Config{
		AuthURL:      manifest.OAuthConfig.AuthURL,
		TokenURL:     manifest.OAuthConfig.TokenURL,
		Scopes:       manifest.OAuthConfig.Scopes,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Tenant:       tenant,
	}, nil
}

// callbackURL derives this server's own OAuth callback URL from the
// incoming request rather than a fixed setting, so it's automatically
// correct for whatever host/scheme the admin is actually browsing on
// (including behind a reverse proxy that sets X-Forwarded-Proto). It
// must resolve identically at both authorize time and callback time,
// which holds as long as the admin reaches the server on one consistent
// hostname -- true for the single-domain self-hosted deployments this
// app targets.
func callbackURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/api/oauth/callback"
}

type authorizeURLResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// handleOAuthAuthorize starts one plugin instance's OAuth2 flow: builds
// the provider's consent URL from its manifest + its own client_id/
// client_secret/tenant, remembers a CSRF state token against this
// instance, and returns the URL as JSON rather than redirecting itself
// -- this endpoint is behind requireAuth (a Bearer header), which a
// plain browser navigation to it can't carry, so the frontend calls it
// via its normal authenticated fetch and then navigates the browser
// itself (window.location = authorize_url) to actually reach the
// provider's consent screen.
func handleOAuthAuthorize(sqldb *sql.DB, registry *plugindata.Registry, pending *oauth.PendingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instanceID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		inst, manifest, err := loadOAuthPlugin(sqldb, registry, instanceID)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cfg, err := oauthConfigFor(manifest, inst.Config, callbackURL(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		state, err := pending.Begin(instanceID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authorizeURLResponse{AuthorizeURL: oauth.BuildAuthURL(cfg, state)})
	}
}

// handleOAuthCallback is the provider's redirect target after the admin
// grants (or denies) consent -- public, since it's invoked by the
// admin's browser navigating away from the provider, not by an
// authenticated API call; the `state` token (single-use, short-lived,
// issued only from handleOAuthAuthorize) is what proves this request is
// legitimate. Always ends in a redirect back to the admin plugin
// instances page, success or failure, with a query param the frontend
// reads to show a toast -- there's no user-facing error page of its own.
func handleOAuthCallback(sqldb *sql.DB, registry *plugindata.Registry, pending *oauth.PendingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if provErr := r.URL.Query().Get("error"); provErr != "" {
			redirectResult(w, r, false, provErr)
			return
		}

		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		instanceID, ok := pending.Consume(state)
		if !ok || code == "" {
			redirectResult(w, r, false, "invalid or expired authorization request")
			return
		}

		inst, manifest, err := loadOAuthPlugin(sqldb, registry, instanceID)
		if err != nil {
			redirectResult(w, r, false, err.Error())
			return
		}
		cfg, err := oauthConfigFor(manifest, inst.Config, callbackURL(r))
		if err != nil {
			redirectResult(w, r, false, err.Error())
			return
		}

		tok, err := oauth.Exchange(r.Context(), cfg, code)
		if err != nil {
			redirectResult(w, r, false, "token exchange failed")
			return
		}

		var refreshToken *string
		if tok.RefreshToken != "" {
			refreshToken = &tok.RefreshToken
		}
		scopes := strings.Join(manifest.OAuthConfig.Scopes, " ")
		if err := db.UpsertOAuthToken(sqldb, instanceID, tok.AccessToken, refreshToken, tok.Expiry, scopes); err != nil {
			redirectResult(w, r, false, "saving token failed")
			return
		}

		redirectResult(w, r, true, "")
	}
}

// handleDeauthorizePluginInstance removes a plugin instance's stored
// OAuth token without deleting the instance itself, so an admin can
// revoke and re-authorize (e.g. after changing scopes or tenant)
// without losing its name/config/schedule.
func handleDeauthorizePluginInstance(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instanceID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := db.DeleteOAuthToken(sqldb, instanceID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func redirectResult(w http.ResponseWriter, r *http.Request, success bool, message string) {
	dest := "/admin/plugins?oauth=success"
	if !success {
		dest = "/admin/plugins?oauth=error&message=" + url.QueryEscape(message)
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

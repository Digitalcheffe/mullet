package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/oauth"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
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

		cfg, err := oauth.ConfigFromManifest(manifest, inst.Config, callbackURL(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		state, codeVerifier, err := pending.Begin(instanceID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authorizeURLResponse{AuthorizeURL: oauth.BuildAuthURL(cfg, state, codeVerifier)})
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
func handleOAuthCallback(sqldb *sql.DB, registry *plugindata.Registry, pending *oauth.PendingStore, sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if provErr := r.URL.Query().Get("error"); provErr != "" {
			redirectResult(w, r, false, provErr)
			return
		}

		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		instanceID, codeVerifier, ok := pending.Consume(state)
		if !ok || code == "" {
			redirectResult(w, r, false, "invalid or expired authorization request")
			return
		}

		inst, manifest, err := loadOAuthPlugin(sqldb, registry, instanceID)
		if err != nil {
			redirectResult(w, r, false, err.Error())
			return
		}
		cfg, err := oauth.ConfigFromManifest(manifest, inst.Config, callbackURL(r))
		if err != nil {
			redirectResult(w, r, false, err.Error())
			return
		}

		tok, err := oauth.Exchange(r.Context(), cfg, code, codeVerifier)
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

		// Without this, a freshly authorized instance keeps showing
		// last tick's "not authorized yet" error (from before this
		// token existed) until its next scheduled tick or an unrelated
		// edit happens to trigger a reload -- same reasoning as every
		// other plugin-instance mutation already reloading immediately.
		if err := sched.Reload(); err != nil {
			log.Printf("instance %d authorized but scheduler reload failed: %v", instanceID, err)
		}

		redirectResult(w, r, true, "")
	}
}

type discoveredOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// handleDiscover backs a "populated after setup" SetupField (any
// plugin's, not just an OAuth2 one -- msgraph-calendar's "calendars"
// needs a live token; home-assistant's "entities" just needs the URL/
// token the admin already typed in, since AuthType "api_key" has no
// separate authorization step at all). For an OAuth2 plugin specifically,
// a fresh access token is injected under "access_token" first, the same
// way the scheduler does before Configure/Fetch -- every other plugin's
// own config already has everything Discover needs, as-is. Runs through
// Registry.WithPlugin like Configure/Fetch, so it can't race a concurrent
// scheduled tick on the same plugin type's shared object.
func handleDiscover(sqldb *sql.DB, registry *plugindata.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instanceID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		field := r.URL.Query().Get("field")
		if field == "" {
			http.Error(w, "field query parameter is required", http.StatusBadRequest)
			return
		}

		inst, err := db.GetPluginInstance(sqldb, instanceID)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		plugin, ok := registry.Get(inst.PluginID)
		if !ok {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		manifest := plugin.Manifest()

		cfg := map[string]any{}
		if err := json.Unmarshal([]byte(inst.Config), &cfg); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if manifest.AuthType == "oauth2" {
			oauthCfg, err := oauth.ConfigFromManifest(manifest, inst.Config, "")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			token, err := oauth.EnsureFreshToken(r.Context(), sqldb, instanceID, oauthCfg)
			if err != nil {
				http.Error(w, "not authorized yet", http.StatusConflict)
				return
			}
			cfg["access_token"] = token
		}

		var options []plugindata.DiscoveredOption
		var discoverErr error
		found, _ := registry.WithPlugin(inst.PluginID, func(p plugindata.DataPlugin) error {
			discoverable, ok := p.(plugindata.Discoverable)
			if !ok {
				discoverErr = fmt.Errorf("plugin %q does not support discovery", inst.PluginID)
				return nil
			}
			options, discoverErr = discoverable.Discover(r.Context(), field, cfg)
			return nil
		})
		if !found {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if discoverErr != nil {
			http.Error(w, discoverErr.Error(), http.StatusBadGateway)
			return
		}

		resp := make([]discoveredOptionResponse, len(options))
		for i, o := range options {
			resp[i] = discoveredOptionResponse{Value: o.Value, Label: o.Label}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handleDeauthorizePluginInstance removes a plugin instance's stored
// OAuth token without deleting the instance itself, so an admin can
// revoke and re-authorize (e.g. after changing scopes or tenant)
// without losing its name/config/schedule.
func handleDeauthorizePluginInstance(sqldb *sql.DB, sched *scheduler.Scheduler) http.HandlerFunc {
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
		if err := sched.Reload(); err != nil {
			log.Printf("instance %d deauthorized but scheduler reload failed: %v", instanceID, err)
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

package oauth

import (
	"encoding/json"
	"errors"
	"fmt"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

// ConfigFromManifest builds a plugin instance's oauth.Config from its
// manifest (auth/token URLs, scopes, which field holds the tenant) and
// its own submitted config (client_id, client_secret, and the tenant
// value) -- the three well-known keys every OAuth2 plugin's manifest is
// expected to declare as SetupFields, the same way openweathermap
// declares "api_key" for its own AuthType. redirectURL is only needed
// for the interactive authorize/callback exchange (unused by a
// refresh-token-only call, since providers don't validate redirect_uri
// on the refresh grant) -- pass "" when building a Config just to
// refresh an existing token.
//
// client_secret is optional: a public client (what Microsoft issues for
// personal/consumer accounts, and increasingly recommends generally) has
// none to submit, and PKCE covers the same request-legitimacy proof a
// secret would (see Config.ClientSecret). Only client_id is required.
func ConfigFromManifest(manifest plugindata.DataPluginManifest, instanceConfigJSON, redirectURL string) (Config, error) {
	if manifest.OAuthConfig == nil {
		return Config{}, errors.New("plugin manifest has no OAuthConfig")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(instanceConfigJSON), &cfg); err != nil {
		return Config{}, fmt.Errorf("instance config is not valid JSON: %w", err)
	}
	clientID, _ := cfg["client_id"].(string)
	clientSecret, _ := cfg["client_secret"].(string)
	if clientID == "" {
		return Config{}, errors.New("client_id must be configured before authorizing")
	}
	var tenant string
	if manifest.OAuthConfig.TenantField != "" {
		tenant, _ = cfg[manifest.OAuthConfig.TenantField].(string)
	}
	return Config{
		AuthURL:      manifest.OAuthConfig.AuthURL,
		TokenURL:     manifest.OAuthConfig.TokenURL,
		Scopes:       manifest.OAuthConfig.Scopes,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Tenant:       tenant,
	}, nil
}

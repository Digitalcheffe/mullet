// Package oauth is the generic, reusable OAuth2 authorization-code flow
// used by any data plugin whose manifest declares AuthType "oauth2"
// (internal/plugins/data.DataPluginManifest.OAuthConfig). It knows
// nothing about a specific provider (Microsoft or otherwise) -- every
// provider-specific detail (auth/token URLs, scopes, and how the tenant
// segment of those URLs is filled in) comes from the manifest plus the
// plugin instance's own config, so a future non-Microsoft OAuth2 plugin
// works the same way without touching this package.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// Config bundles everything one plugin instance's OAuth2 flow needs,
// assembled by the caller from the plugin's manifest (AuthURL, TokenURL,
// Scopes, and which SetupField holds the tenant) and the instance's own
// config (client_id, client_secret, and the tenant value itself).
type Config struct {
	AuthURL      string
	TokenURL     string
	Scopes       []string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// Tenant fills a literal "{tenant}" placeholder in AuthURL/TokenURL,
	// if either contains one -- Microsoft's identity platform URLs are
	// tenant-scoped (e.g. ".../{tenant}/oauth2/v2.0/authorize", where
	// tenant is "consumers", "organizations", "common", or a tenant
	// GUID); a provider whose URLs aren't tenant-scoped just leaves this
	// empty and the placeholder (absent from its URLs) is never used.
	Tenant string
}

func (c Config) resolvedAuthURL() string { return strings.ReplaceAll(c.AuthURL, "{tenant}", c.Tenant) }
func (c Config) resolvedTokenURL() string {
	return strings.ReplaceAll(c.TokenURL, "{tenant}", c.Tenant)
}

func (c Config) toOAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Scopes:       c.Scopes,
		RedirectURL:  c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.resolvedAuthURL(),
			TokenURL: c.resolvedTokenURL(),
		},
	}
}

// NewState generates an unpredictable CSRF state token for one
// authorization attempt. The caller is responsible for remembering
// which plugin instance it belongs to (see PendingStore) and validating
// it on the callback -- this package has no notion of "instance" itself.
func NewState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating oauth state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// BuildAuthURL returns the URL to send the admin's browser to, offline
// access requested (so the response includes a refresh token) via
// AccessTypeOffline -- harmless for providers that ignore the param,
// required by Google-style providers that would otherwise omit it.
func BuildAuthURL(cfg Config, state string) string {
	return cfg.toOAuth2Config().AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange trades an authorization code (from the callback's ?code=)
// for an access/refresh token pair.
func Exchange(ctx context.Context, cfg Config, code string) (*oauth2.Token, error) {
	tok, err := cfg.toOAuth2Config().Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging authorization code: %w", err)
	}
	return tok, nil
}

// Refresh trades a refresh token for a new access token.
func Refresh(ctx context.Context, cfg Config, refreshToken string) (*oauth2.Token, error) {
	src := cfg.toOAuth2Config().TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	tok, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	return tok, nil
}

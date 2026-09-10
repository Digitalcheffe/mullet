package oauth

import (
	"testing"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

func testManifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		AuthType: "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{
			AuthURL:     "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize",
			TokenURL:    "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token",
			Scopes:      []string{"Calendars.Read"},
			TenantField: "tenant",
		},
	}
}

// TestConfigFromManifestAllowsMissingClientSecret guards issue #77: a
// public client (what Microsoft issues for personal/consumer accounts)
// has no client_secret to submit, so it must not be required.
func TestConfigFromManifestAllowsMissingClientSecret(t *testing.T) {
	cfg, err := ConfigFromManifest(testManifest(), `{"client_id":"abc","tenant":"consumers"}`, "https://mullet.example/api/oauth/callback")
	if err != nil {
		t.Fatalf("ConfigFromManifest: %v", err)
	}
	if cfg.ClientID != "abc" {
		t.Errorf("ClientID = %q, want abc", cfg.ClientID)
	}
	if cfg.ClientSecret != "" {
		t.Errorf("ClientSecret = %q, want empty", cfg.ClientSecret)
	}
	if cfg.Tenant != "consumers" {
		t.Errorf("Tenant = %q, want consumers", cfg.Tenant)
	}
}

func TestConfigFromManifestStillCarriesClientSecretWhenPresent(t *testing.T) {
	cfg, err := ConfigFromManifest(testManifest(), `{"client_id":"abc","client_secret":"xyz","tenant":"organizations"}`, "")
	if err != nil {
		t.Fatalf("ConfigFromManifest: %v", err)
	}
	if cfg.ClientSecret != "xyz" {
		t.Errorf("ClientSecret = %q, want xyz (a confidential-client registration must still work)", cfg.ClientSecret)
	}
}

func TestConfigFromManifestRequiresClientID(t *testing.T) {
	if _, err := ConfigFromManifest(testManifest(), `{"client_secret":"xyz","tenant":"consumers"}`, ""); err == nil {
		t.Error("ConfigFromManifest with no client_id = nil error, want failure")
	}
}

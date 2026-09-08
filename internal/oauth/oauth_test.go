package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBuildAuthURLSubstitutesTenantAndCarriesScopes(t *testing.T) {
	cfg := Config{
		AuthURL:      "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize",
		TokenURL:     "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token",
		Scopes:       []string{"Calendars.Read", "offline_access"},
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
		RedirectURL:  "https://mullet.example/api/oauth/callback",
		Tenant:       "consumers",
	}

	authURL := BuildAuthURL(cfg, "state-123")
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parsing auth URL: %v", err)
	}
	if !strings.HasPrefix(authURL, "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize") {
		t.Errorf("authURL = %q, want tenant substituted into the path", authURL)
	}
	q := parsed.Query()
	if q.Get("client_id") != "client-abc" {
		t.Errorf("client_id = %q, want client-abc", q.Get("client_id"))
	}
	if q.Get("state") != "state-123" {
		t.Errorf("state = %q, want state-123", q.Get("state"))
	}
	if q.Get("redirect_uri") != cfg.RedirectURL {
		t.Errorf("redirect_uri = %q, want %q", q.Get("redirect_uri"), cfg.RedirectURL)
	}
	if got := q.Get("scope"); !strings.Contains(got, "Calendars.Read") || !strings.Contains(got, "offline_access") {
		t.Errorf("scope = %q, want both requested scopes", got)
	}
}

func TestBuildAuthURLWithoutTenantLeavesURLUnchanged(t *testing.T) {
	cfg := Config{
		AuthURL:  "https://example.com/oauth/authorize",
		TokenURL: "https://example.com/oauth/token",
		ClientID: "client-abc",
	}
	authURL := BuildAuthURL(cfg, "s")
	if !strings.HasPrefix(authURL, "https://example.com/oauth/authorize") {
		t.Errorf("authURL = %q, want unchanged host/path for a non-tenant-scoped provider", authURL)
	}
}

// fakeTokenServer stands in for a provider's token endpoint, responding
// to both the authorization_code and refresh_token grants a real
// provider like Microsoft's identity platform supports.
func fakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing token request form: %v", err)
		}
		grant := r.Form.Get("grant_type")
		w.Header().Set("Content-Type", "application/json")
		switch grant {
		case "authorization_code":
			if r.Form.Get("code") != "auth-code-1" {
				http.Error(w, "unexpected code", http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-from-code", "refresh_token": "refresh-1",
				"token_type": "Bearer", "expires_in": 3600,
			})
		case "refresh_token":
			if r.Form.Get("refresh_token") != "refresh-1" {
				http.Error(w, "unexpected refresh_token", http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-refreshed", "refresh_token": "refresh-2",
				"token_type": "Bearer", "expires_in": 3600,
			})
		default:
			http.Error(w, "unexpected grant_type", http.StatusBadRequest)
		}
	}))
}

func TestExchange(t *testing.T) {
	srv := fakeTokenServer(t)
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "client-abc", ClientSecret: "secret-xyz"}
	tok, err := Exchange(t.Context(), cfg, "auth-code-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "access-from-code" || tok.RefreshToken != "refresh-1" {
		t.Errorf("tok = %+v, unexpected values", tok)
	}
}

func TestExchangeWrongCodeFails(t *testing.T) {
	srv := fakeTokenServer(t)
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "client-abc", ClientSecret: "secret-xyz"}
	if _, err := Exchange(t.Context(), cfg, "wrong-code"); err == nil {
		t.Error("Exchange(wrong code) = nil error, want failure")
	}
}

func TestRefresh(t *testing.T) {
	srv := fakeTokenServer(t)
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "client-abc", ClientSecret: "secret-xyz"}
	tok, err := Refresh(t.Context(), cfg, "refresh-1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.AccessToken != "access-refreshed" {
		t.Errorf("AccessToken = %q, want access-refreshed", tok.AccessToken)
	}
}

func TestNewStateIsUnpredictableAndNonEmpty(t *testing.T) {
	a, err := NewState()
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	b, err := NewState()
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("NewState returned an empty string")
	}
	if a == b {
		t.Error("two calls to NewState returned the same token")
	}
}

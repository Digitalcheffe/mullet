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

	authURL := BuildAuthURL(cfg, "state-123", "verifier-abc")
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
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge is empty, want a PKCE challenge derived from the verifier")
	}
}

func TestBuildAuthURLWithoutTenantLeavesURLUnchanged(t *testing.T) {
	cfg := Config{
		AuthURL:  "https://example.com/oauth/authorize",
		TokenURL: "https://example.com/oauth/token",
		ClientID: "client-abc",
	}
	authURL := BuildAuthURL(cfg, "s", "verifier")
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
	tok, err := Exchange(t.Context(), cfg, "auth-code-1", "verifier-xyz")
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
	if _, err := Exchange(t.Context(), cfg, "wrong-code", "verifier-xyz"); err == nil {
		t.Error("Exchange(wrong code) = nil error, want failure")
	}
}

// TestExchangeSendsCodeVerifierAndOmitsSecretForPublicClient guards the
// two changes issue #77 depends on together: PKCE's code_verifier must
// reach the token request, and a public client (Config.ClientSecret left
// empty, the only kind Microsoft issues for personal/consumer accounts)
// must not send an empty client_secret param in its place.
func TestExchangeSendsCodeVerifierAndOmitsSecretForPublicClient(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing token request form: %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-from-code", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "client-abc"} // no ClientSecret
	if _, err := Exchange(t.Context(), cfg, "auth-code-1", "verifier-xyz"); err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if gotForm.Get("code_verifier") != "verifier-xyz" {
		t.Errorf("code_verifier = %q, want verifier-xyz", gotForm.Get("code_verifier"))
	}
	if _, present := gotForm["client_secret"]; present {
		t.Errorf("client_secret param present (value %q), want it omitted entirely for a public client", gotForm.Get("client_secret"))
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

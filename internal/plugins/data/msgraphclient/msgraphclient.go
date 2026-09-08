// Package msgraphclient builds an authenticated Microsoft Graph SDK
// client from a plain access token string. Shared by the msgraphcalendar
// and msgraphtodo plugins -- separate plugins, same OAuth2 credential
// plumbing (this project already owns token acquisition/refresh via
// internal/oauth, so there's no MSAL device-code/interactive flow here,
// just wrapping the already-fresh token in the interface Kiota's own
// auth provider expects).
package msgraphclient

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	kiotaauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// staticTokenCredential satisfies azcore.TokenCredential by always
// returning the same pre-acquired token -- the SDK's normal credential
// types (device code, client secret, ...) each perform their own token
// acquisition; this one deliberately does nothing but hand back a token
// this project already fetched and refreshed itself.
type staticTokenCredential struct{ token string }

func (c staticTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// ExpiresOn is a hint the SDK's own retry logic may consult; it isn't
	// what governs whether the token actually works, so a token that's
	// nearer to expiry than this still succeeds or fails exactly as
	// Microsoft's API decides -- freshness is internal/oauth.EnsureFreshToken's
	// job, upstream of this package, not this credential's.
	return azcore.AccessToken{Token: c.token, ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// New builds a Graph client authenticated with accessToken, talking to
// the real Microsoft Graph API.
func New(accessToken string) (*msgraphsdk.GraphServiceClient, error) {
	return build(accessToken, "")
}

// NewWithBaseURL is New with the SDK's base URL overridden -- exists so
// tests can point it at a fake local server instead of
// graph.microsoft.com, without needing a real Microsoft account.
func NewWithBaseURL(accessToken, baseURL string) (*msgraphsdk.GraphServiceClient, error) {
	return build(accessToken, baseURL)
}

func build(accessToken, baseURL string) (*msgraphsdk.GraphServiceClient, error) {
	authProvider, err := kiotaauth.NewAzureIdentityAuthenticationProvider(staticTokenCredential{token: accessToken})
	if err != nil {
		return nil, fmt.Errorf("creating graph auth provider: %w", err)
	}
	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, fmt.Errorf("creating graph request adapter: %w", err)
	}
	if baseURL != "" {
		adapter.SetBaseUrl(baseURL)
	}
	return msgraphsdk.NewGraphServiceClient(adapter), nil
}

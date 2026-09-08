package oauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// expiryLeeway refreshes a token this long before its actual expiry, so
// a plugin's Fetch call never races a token that's still technically
// valid when EnsureFreshToken checks it but expires mid-request.
const expiryLeeway = 2 * time.Minute

// EnsureFreshToken returns a valid access token for a plugin instance,
// transparently refreshing and persisting a new one if the stored token
// is expired (or expiring soon) and a refresh token is available. This
// is what a plugin's Configure step (or the scheduler, on its behalf)
// calls before each Fetch -- returns db.ErrNotFound if the instance was
// never authorized.
func EnsureFreshToken(ctx context.Context, sqldb *sql.DB, instanceID int, cfg Config) (string, error) {
	tok, err := db.GetOAuthToken(sqldb, instanceID)
	if err != nil {
		return "", err
	}

	if time.Now().Add(expiryLeeway).Before(tok.ExpiresAt) {
		return tok.AccessToken, nil
	}
	if tok.RefreshToken == nil {
		return "", fmt.Errorf("oauth token for instance %d expired and has no refresh token -- re-authorize", instanceID)
	}

	refreshed, err := Refresh(ctx, cfg, *tok.RefreshToken)
	if err != nil {
		return "", err
	}

	// A provider isn't required to rotate the refresh token on every
	// use -- keep the old one unless a new one came back.
	newRefresh := tok.RefreshToken
	if refreshed.RefreshToken != "" {
		newRefresh = &refreshed.RefreshToken
	}
	if err := db.UpsertOAuthToken(sqldb, instanceID, refreshed.AccessToken, newRefresh, refreshed.Expiry, tok.Scopes); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

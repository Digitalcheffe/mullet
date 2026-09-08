package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// OAuthToken is one plugin instance's stored OAuth2 credentials.
// RefreshToken is nil for a provider that didn't grant one (e.g. no
// "offline_access" scope) -- EnsureFreshToken treats that as
// unrefreshable once ExpiresAt passes.
type OAuthToken struct {
	PluginInstanceID int
	AccessToken      string
	RefreshToken     *string
	ExpiresAt        time.Time
	Scopes           string
}

// GetOAuthToken returns the stored token for a plugin instance,
// decrypted, or ErrNotFound if it was never authorized.
func GetOAuthToken(sqldb *sql.DB, instanceID int) (OAuthToken, error) {
	row := sqldb.QueryRow(
		`SELECT plugin_instance_id, access_token, refresh_token, expires_at, scopes
		 FROM oauth_tokens WHERE plugin_instance_id = ?`,
		instanceID,
	)
	var t OAuthToken
	if err := row.Scan(&t.PluginInstanceID, &t.AccessToken, &t.RefreshToken, &t.ExpiresAt, &t.Scopes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OAuthToken{}, ErrNotFound
		}
		return OAuthToken{}, fmt.Errorf("getting oauth token for instance %d: %w", instanceID, err)
	}

	key, err := loadOrCreateOAuthEncryptionKey(sqldb)
	if err != nil {
		return OAuthToken{}, err
	}
	if t.AccessToken, err = decryptString(key, t.AccessToken); err != nil {
		return OAuthToken{}, fmt.Errorf("decrypting access token for instance %d: %w", instanceID, err)
	}
	if t.RefreshToken != nil {
		decrypted, err := decryptString(key, *t.RefreshToken)
		if err != nil {
			return OAuthToken{}, fmt.Errorf("decrypting refresh token for instance %d: %w", instanceID, err)
		}
		t.RefreshToken = &decrypted
	}
	return t, nil
}

// HasOAuthToken reports whether a plugin instance has been authorized
// (has any stored token, expired or not) without fetching the token
// itself -- what the plugin instances list needs to render an
// "Authorize" vs "Re-authorize" button.
func HasOAuthToken(sqldb *sql.DB, instanceID int) (bool, error) {
	var exists int
	err := sqldb.QueryRow(`SELECT 1 FROM oauth_tokens WHERE plugin_instance_id = ?`, instanceID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking oauth token for instance %d: %w", instanceID, err)
	}
	return true, nil
}

// UpsertOAuthToken encrypts and stores the token for a plugin instance,
// replacing whatever was there before -- called after both the initial
// authorize callback and every subsequent refresh.
func UpsertOAuthToken(sqldb *sql.DB, instanceID int, accessToken string, refreshToken *string, expiresAt time.Time, scopes string) error {
	key, err := loadOrCreateOAuthEncryptionKey(sqldb)
	if err != nil {
		return err
	}
	encryptedAccess, err := encryptString(key, accessToken)
	if err != nil {
		return fmt.Errorf("encrypting access token for instance %d: %w", instanceID, err)
	}
	var encryptedRefresh *string
	if refreshToken != nil {
		enc, err := encryptString(key, *refreshToken)
		if err != nil {
			return fmt.Errorf("encrypting refresh token for instance %d: %w", instanceID, err)
		}
		encryptedRefresh = &enc
	}

	_, err = sqldb.Exec(
		`INSERT INTO oauth_tokens (plugin_instance_id, access_token, refresh_token, expires_at, scopes, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(plugin_instance_id) DO UPDATE SET
		   access_token = excluded.access_token,
		   refresh_token = excluded.refresh_token,
		   expires_at = excluded.expires_at,
		   scopes = excluded.scopes,
		   updated_at = CURRENT_TIMESTAMP`,
		instanceID, encryptedAccess, encryptedRefresh, expiresAt.UTC(), scopes,
	)
	if err != nil {
		return fmt.Errorf("saving oauth token for instance %d: %w", instanceID, err)
	}
	return nil
}

// DeleteOAuthToken removes a plugin instance's stored token, if any (not
// an error if there wasn't one) -- lets an admin de-authorize without
// deleting the instance itself.
func DeleteOAuthToken(sqldb *sql.DB, instanceID int) error {
	if _, err := sqldb.Exec(`DELETE FROM oauth_tokens WHERE plugin_instance_id = ?`, instanceID); err != nil {
		return fmt.Errorf("deleting oauth token for instance %d: %w", instanceID, err)
	}
	return nil
}

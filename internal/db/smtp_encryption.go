package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
)

const smtpEncryptionKeySettingKey = "smtp_password_encryption_key"

// loadOrCreateSMTPEncryptionKey returns the server's AES-256 key for
// encrypting the SMTP password at rest, generating and persisting a new
// random one on first use so it survives restarts -- same pattern as
// loadOrCreateOAuthEncryptionKey (oauth_encryption.go), just under its
// own system_settings key. Kept separate from the OAuth key rather than
// reused: two unrelated secret types shouldn't share a key just because
// the mechanism is the same, and each new secret type introduced so far
// (JWT signing, OAuth tokens) has minted its own.
func loadOrCreateSMTPEncryptionKey(sqldb *sql.DB) ([]byte, error) {
	existing, found, err := GetSetting(sqldb, smtpEncryptionKeySettingKey)
	if err != nil {
		return nil, err
	}
	if found {
		key, err := base64.StdEncoding.DecodeString(existing)
		if err != nil {
			return nil, fmt.Errorf("decoding stored smtp encryption key: %w", err)
		}
		return key, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating smtp encryption key: %w", err)
	}
	if err := SetSetting(sqldb, smtpEncryptionKeySettingKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

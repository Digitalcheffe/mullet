package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
)

const webhookEncryptionKeySettingKey = "webhook_secret_encryption_key"

// loadOrCreateWebhookEncryptionKey returns the server's AES-256 key for
// encrypting the webhook signing secret at rest -- its own key, minted
// on first use, same pattern as loadOrCreateSMTPEncryptionKey.
func loadOrCreateWebhookEncryptionKey(sqldb *sql.DB) ([]byte, error) {
	existing, found, err := GetSetting(sqldb, webhookEncryptionKeySettingKey)
	if err != nil {
		return nil, err
	}
	if found {
		key, err := base64.StdEncoding.DecodeString(existing)
		if err != nil {
			return nil, fmt.Errorf("decoding stored webhook encryption key: %w", err)
		}
		return key, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating webhook encryption key: %w", err)
	}
	if err := SetSetting(sqldb, webhookEncryptionKeySettingKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

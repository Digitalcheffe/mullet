package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
)

const totpEncryptionKeySettingKey = "totp_secret_encryption_key"

// loadOrCreateTOTPEncryptionKey returns the server's AES-256 key for
// encrypting TOTP secrets at rest -- its own key, minted on first use,
// same pattern as loadOrCreateSMTPEncryptionKey.
func loadOrCreateTOTPEncryptionKey(sqldb *sql.DB) ([]byte, error) {
	existing, found, err := GetSetting(sqldb, totpEncryptionKeySettingKey)
	if err != nil {
		return nil, err
	}
	if found {
		key, err := base64.StdEncoding.DecodeString(existing)
		if err != nil {
			return nil, fmt.Errorf("decoding stored totp encryption key: %w", err)
		}
		return key, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating totp encryption key: %w", err)
	}
	if err := SetSetting(sqldb, totpEncryptionKeySettingKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

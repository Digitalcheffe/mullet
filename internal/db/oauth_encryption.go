package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
)

const oauthEncryptionKeySettingKey = "oauth_token_encryption_key"

// loadOrCreateOAuthEncryptionKey returns the server's AES-256 key for
// encrypting OAuth2 tokens at rest, generating and persisting a new
// random one on first use so it survives restarts -- same pattern as
// auth.LoadOrCreateJWTSecret, just under its own system_settings key.
func loadOrCreateOAuthEncryptionKey(sqldb *sql.DB) ([]byte, error) {
	existing, found, err := GetSetting(sqldb, oauthEncryptionKeySettingKey)
	if err != nil {
		return nil, err
	}
	if found {
		key, err := base64.StdEncoding.DecodeString(existing)
		if err != nil {
			return nil, fmt.Errorf("decoding stored oauth encryption key: %w", err)
		}
		return key, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating oauth encryption key: %w", err)
	}
	if err := SetSetting(sqldb, oauthEncryptionKeySettingKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

// encryptString encrypts plaintext with AES-256-GCM, returning
// base64(nonce || ciphertext). The nonce isn't secret, just unique per
// encryption -- prefixing it onto the ciphertext (rather than storing it
// separately) is the standard approach, and it self-describes the
// boundary decryptString needs.
func encryptString(key []byte, plaintext string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptString reverses encryptString.
func decryptString(key []byte, encoded string) (string, error) {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("ciphertext shorter than nonce")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting (wrong key, or ciphertext tampered with): %w", err)
	}
	return string(plaintext), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return gcm, nil
}

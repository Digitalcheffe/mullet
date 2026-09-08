package db

import (
	"testing"
	"time"
)

func TestEncryptDecryptStringRoundTrips(t *testing.T) {
	sqldb := newTestDB(t)
	key, err := loadOrCreateOAuthEncryptionKey(sqldb)
	if err != nil {
		t.Fatalf("loadOrCreateOAuthEncryptionKey: %v", err)
	}

	encrypted, err := encryptString(key, "super-secret-token")
	if err != nil {
		t.Fatalf("encryptString: %v", err)
	}
	if encrypted == "super-secret-token" {
		t.Fatal("encryptString returned the plaintext unchanged")
	}

	decrypted, err := decryptString(key, encrypted)
	if err != nil {
		t.Fatalf("decryptString: %v", err)
	}
	if decrypted != "super-secret-token" {
		t.Errorf("decrypted = %q, want super-secret-token", decrypted)
	}
}

func TestEncryptStringIsNonDeterministic(t *testing.T) {
	sqldb := newTestDB(t)
	key, _ := loadOrCreateOAuthEncryptionKey(sqldb)

	a, err := encryptString(key, "same-plaintext")
	if err != nil {
		t.Fatalf("encryptString: %v", err)
	}
	b, err := encryptString(key, "same-plaintext")
	if err != nil {
		t.Fatalf("encryptString: %v", err)
	}
	if a == b {
		t.Error("encrypting the same plaintext twice produced identical ciphertext (nonce not varying)")
	}
}

func TestDecryptStringWithWrongKeyFails(t *testing.T) {
	sqldb := newTestDB(t)
	key, _ := loadOrCreateOAuthEncryptionKey(sqldb)
	encrypted, _ := encryptString(key, "secret")

	wrongKey := make([]byte, 32)
	for i := range wrongKey {
		wrongKey[i] = byte(i)
	}
	if wrongKey[0] == key[0] {
		wrongKey[0]++
	}

	if _, err := decryptString(wrongKey, encrypted); err == nil {
		t.Error("decryptString with the wrong key succeeded, want failure")
	}
}

func TestLoadOrCreateOAuthEncryptionKeyPersists(t *testing.T) {
	sqldb := newTestDB(t)

	first, err := loadOrCreateOAuthEncryptionKey(sqldb)
	if err != nil {
		t.Fatalf("loadOrCreateOAuthEncryptionKey (first): %v", err)
	}
	if len(first) != 32 {
		t.Fatalf("key length = %d, want 32 (AES-256)", len(first))
	}

	second, err := loadOrCreateOAuthEncryptionKey(sqldb)
	if err != nil {
		t.Fatalf("loadOrCreateOAuthEncryptionKey (second): %v", err)
	}
	if string(first) != string(second) {
		t.Error("second call generated a different key instead of reusing the persisted one")
	}
}

// TestOAuthTokensStoredEncryptedNotPlaintext guards the actual point of
// this package: the raw DB row must never contain the plaintext token,
// even though GetOAuthToken transparently decrypts it back for callers.
func TestOAuthTokensStoredEncryptedNotPlaintext(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := CreatePluginInstance(sqldb, "msgraph-calendar", "Work", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	refresh := "refresh-plaintext-marker"
	if err := UpsertOAuthToken(sqldb, instanceID, "access-plaintext-marker", &refresh, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	var rawAccess, rawRefresh string
	row := sqldb.QueryRow(`SELECT access_token, refresh_token FROM oauth_tokens WHERE plugin_instance_id = ?`, instanceID)
	if err := row.Scan(&rawAccess, &rawRefresh); err != nil {
		t.Fatalf("scanning raw row: %v", err)
	}
	if rawAccess == "access-plaintext-marker" {
		t.Error("access_token stored in plaintext")
	}
	if rawRefresh == "refresh-plaintext-marker" {
		t.Error("refresh_token stored in plaintext")
	}

	got, err := GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got.AccessToken != "access-plaintext-marker" || got.RefreshToken == nil || *got.RefreshToken != "refresh-plaintext-marker" {
		t.Errorf("GetOAuthToken didn't transparently decrypt: got = %+v", got)
	}
}

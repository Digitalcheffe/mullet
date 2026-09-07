package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/Digitalcheffe/mullet/internal/db"
)

const jwtSecretSettingKey = "jwt_secret"

// LoadOrCreateJWTSecret returns the server's JWT signing secret, generating
// and persisting a new random one on first run so tokens survive restarts
// without requiring the user to manage a secret manually.
func LoadOrCreateJWTSecret(sqldb *sql.DB) ([]byte, error) {
	existing, found, err := db.GetSetting(sqldb, jwtSecretSettingKey)
	if err != nil {
		return nil, err
	}
	if found {
		secret, err := base64.StdEncoding.DecodeString(existing)
		if err != nil {
			return nil, fmt.Errorf("decoding stored jwt secret: %w", err)
		}
		return secret, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating jwt secret: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(raw)
	if err := db.SetSetting(sqldb, jwtSecretSettingKey, encoded); err != nil {
		return nil, err
	}

	return raw, nil
}

package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const passwordResetTokenTTL = time.Hour

// ErrResetTokenInvalid covers every way a reset token can fail to
// redeem: unknown, expired, or already used. Deliberately one error for
// all three -- distinguishing them to the caller would only help an
// attacker probe which case applies to a given token.
var ErrResetTokenInvalid = errors.New("reset token is invalid, expired, or already used")

// CreatePasswordResetToken generates a new single-use token for userID,
// valid for one hour, and returns the raw token to email to the account
// (see internal/api/password_reset_handlers.go). Only the token's
// SHA-256 hash is stored -- a database leak alone can't be used to
// redeem it, since the raw value never touches disk.
func CreatePasswordResetToken(sqldb *sql.DB, userID int) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	_, err := sqldb.Exec(
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, hashResetToken(token), time.Now().Add(passwordResetTokenTTL),
	)
	if err != nil {
		return "", fmt.Errorf("storing reset token: %w", err)
	}
	return token, nil
}

// ConsumePasswordResetToken redeems token: if it exists, hasn't expired,
// and hasn't already been used, it's marked used and the associated
// user ID is returned. Otherwise returns ErrResetTokenInvalid. Marking
// used happens as part of the same call (not a separate step) so a
// token can never be validated twice, even by two concurrent requests
// racing each other -- the second one's UPDATE affects zero rows.
func ConsumePasswordResetToken(sqldb *sql.DB, token string) (int, error) {
	result, err := sqldb.Exec(
		`UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		time.Now(), hashResetToken(token), time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("redeeming reset token: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("checking redeemed reset token: %w", err)
	}
	if n == 0 {
		return 0, ErrResetTokenInvalid
	}

	var userID int
	if err := sqldb.QueryRow(`SELECT user_id FROM password_reset_tokens WHERE token_hash = ?`, hashResetToken(token)).Scan(&userID); err != nil {
		return 0, fmt.Errorf("looking up redeemed reset token's user: %w", err)
	}
	return userID, nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"time"
)

const backupCodeCount = 10

// StartTOTPEnrollment stages a new (unconfirmed) TOTP secret for
// userID, encrypted at rest, without enabling it -- ConfirmTOTPEnrollment
// must see one valid code against this exact secret before login starts
// requiring it. Callers must refuse to call this while TOTP is already
// enabled (see handleEnrollTOTP): overwriting an active secret out from
// under a confirmed enrollment would risk locking the account out for no
// benefit, since disabling first is always available as an explicit step.
func StartTOTPEnrollment(sqldb *sql.DB, userID int, secret string) error {
	key, err := loadOrCreateTOTPEncryptionKey(sqldb)
	if err != nil {
		return err
	}
	encrypted, err := encryptString(key, secret)
	if err != nil {
		return err
	}
	result, err := sqldb.Exec(`UPDATE users SET totp_secret = ?, totp_enabled = 0 WHERE id = ?`, encrypted, userID)
	if err != nil {
		return fmt.Errorf("staging totp secret for user %d: %w", userID, err)
	}
	return checkRowsAffected(result, userID)
}

// ConfirmTOTPEnrollment flips totp_enabled on for userID -- call only
// after verifying a code against the secret StartTOTPEnrollment staged.
func ConfirmTOTPEnrollment(sqldb *sql.DB, userID int) error {
	result, err := sqldb.Exec(`UPDATE users SET totp_enabled = 1 WHERE id = ? AND totp_secret IS NOT NULL`, userID)
	if err != nil {
		return fmt.Errorf("confirming totp enrollment for user %d: %w", userID, err)
	}
	return checkRowsAffected(result, userID)
}

// DisableTOTP clears userID's secret, disables it, and deletes all of
// its backup codes -- a half-disabled state (enabled=0 but a secret and
// backup codes still on file) would be confusing and serves no purpose.
func DisableTOTP(sqldb *sql.DB, userID int) error {
	tx, err := sqldb.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE users SET totp_secret = NULL, totp_enabled = 0 WHERE id = ?`, userID); err != nil {
		return fmt.Errorf("disabling totp for user %d: %w", userID, err)
	}
	if _, err := tx.Exec(`DELETE FROM totp_backup_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("deleting backup codes for user %d: %w", userID, err)
	}
	return tx.Commit()
}

// GetTOTPSecret returns userID's decrypted TOTP secret, whatever its
// totp_enabled state -- used both to verify a not-yet-confirmed
// enrollment's first code and to verify a code at login once enabled.
// Returns ErrNotFound if no secret has ever been staged.
func GetTOTPSecret(sqldb *sql.DB, userID int) (string, error) {
	var encrypted sql.NullString
	err := sqldb.QueryRow(`SELECT totp_secret FROM users WHERE id = ?`, userID).Scan(&encrypted)
	if err == sql.ErrNoRows || (err == nil && !encrypted.Valid) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("getting totp secret for user %d: %w", userID, err)
	}

	key, err := loadOrCreateTOTPEncryptionKey(sqldb)
	if err != nil {
		return "", err
	}
	return decryptString(key, encrypted.String)
}

// GenerateBackupCodes creates backupCodeCount fresh one-time recovery
// codes for userID, replacing any it already had (regenerating
// invalidates the old set, since a code shown once and possibly written
// down somewhere shouldn't keep working alongside a fresh batch). The
// plaintext codes are returned for display exactly once -- only their
// SHA-256 hashes are stored.
func GenerateBackupCodes(sqldb *sql.DB, userID int) ([]string, error) {
	tx, err := sqldb.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM totp_backup_codes WHERE user_id = ?`, userID); err != nil {
		return nil, fmt.Errorf("clearing old backup codes for user %d: %w", userID, err)
	}

	codes := make([]string, backupCodeCount)
	for i := range codes {
		code, err := generateBackupCode()
		if err != nil {
			return nil, err
		}
		codes[i] = code
		if _, err := tx.Exec(
			`INSERT INTO totp_backup_codes (user_id, code_hash) VALUES (?, ?)`,
			userID, hashBackupCode(code),
		); err != nil {
			return nil, fmt.Errorf("storing backup code for user %d: %w", userID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing backup codes for user %d: %w", userID, err)
	}
	return codes, nil
}

// ConsumeBackupCode redeems one of userID's backup codes, marking it
// used so it can never be redeemed again. Reports whether code matched
// an unused one.
func ConsumeBackupCode(sqldb *sql.DB, userID int, code string) (bool, error) {
	result, err := sqldb.Exec(
		`UPDATE totp_backup_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`,
		time.Now(), userID, hashBackupCode(code),
	)
	if err != nil {
		return false, fmt.Errorf("redeeming backup code for user %d: %w", userID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking redeemed backup code for user %d: %w", userID, err)
	}
	return n > 0, nil
}

// CountRemainingBackupCodes returns how many of userID's backup codes
// are still unused.
func CountRemainingBackupCodes(sqldb *sql.DB, userID int) (int, error) {
	var count int
	err := sqldb.QueryRow(
		`SELECT COUNT(*) FROM totp_backup_codes WHERE user_id = ? AND used_at IS NULL`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting backup codes for user %d: %w", userID, err)
	}
	return count, nil
}

// generateBackupCode returns one recovery code, formatted XXXX-XXXX
// from base32's digits-and-uppercase-letters alphabet for easy manual
// transcription.
func generateBackupCode() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating backup code: %w", err)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return encoded[:4] + "-" + encoded[4:8], nil
}

func hashBackupCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

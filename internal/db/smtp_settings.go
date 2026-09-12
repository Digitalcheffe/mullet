package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const smtpConfigSettingKey = "smtp_config"

// TLS modes for SMTPConfig.TLSMode.
const (
	SMTPTLSNone     = "none"     // no encryption -- only for a trusted local relay
	SMTPTLSStartTLS = "starttls" // plaintext connection upgraded via STARTTLS (port 587 convention)
	SMTPTLSTLS      = "tls"      // implicit TLS from the first byte (port 465 convention)
)

// SMTPConfig is the server's outgoing-mail configuration (issue #78).
// Password is always the plaintext value here -- GetSMTPConfig decrypts
// it before returning, and SaveSMTPConfig encrypts it before storing, so
// nothing outside this file ever handles the encrypted form.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	TLSMode     string
}

// storedSMTPConfig is SMTPConfig's on-disk shape -- the only difference
// is the password field, which is encrypted at rest the same way an
// OAuth2 refresh token is (see oauth_encryption.go).
type storedSMTPConfig struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	EncryptedPassword string `json:"encrypted_password"`
	FromAddress       string `json:"from_address"`
	TLSMode           string `json:"tls_mode"`
}

// GetSMTPConfig returns the server's saved SMTP configuration, or
// found=false if none has ever been saved.
func GetSMTPConfig(sqldb *sql.DB) (SMTPConfig, bool, error) {
	raw, found, err := GetSetting(sqldb, smtpConfigSettingKey)
	if err != nil {
		return SMTPConfig{}, false, err
	}
	if !found {
		return SMTPConfig{}, false, nil
	}

	var stored storedSMTPConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return SMTPConfig{}, false, fmt.Errorf("decoding stored smtp config: %w", err)
	}

	password := ""
	if stored.EncryptedPassword != "" {
		key, err := loadOrCreateSMTPEncryptionKey(sqldb)
		if err != nil {
			return SMTPConfig{}, false, err
		}
		password, err = decryptString(key, stored.EncryptedPassword)
		if err != nil {
			return SMTPConfig{}, false, err
		}
	}

	return SMTPConfig{
		Host: stored.Host, Port: stored.Port, Username: stored.Username,
		Password: password, FromAddress: stored.FromAddress, TLSMode: stored.TLSMode,
	}, true, nil
}

// SaveSMTPConfig persists cfg, encrypting the password before storing it.
// An empty cfg.Password keeps whatever password was already saved --
// lets the admin update the host/port/etc. from Settings without having
// to re-type the password every time (the frontend never receives the
// existing password back to resubmit, so this can't mean "clear it").
func SaveSMTPConfig(sqldb *sql.DB, cfg SMTPConfig) error {
	key, err := loadOrCreateSMTPEncryptionKey(sqldb)
	if err != nil {
		return err
	}

	encryptedPassword := ""
	if cfg.Password != "" {
		encryptedPassword, err = encryptString(key, cfg.Password)
		if err != nil {
			return err
		}
	} else if existingRaw, found, err := GetSetting(sqldb, smtpConfigSettingKey); err != nil {
		return err
	} else if found {
		var existing storedSMTPConfig
		if err := json.Unmarshal([]byte(existingRaw), &existing); err == nil {
			encryptedPassword = existing.EncryptedPassword
		}
	}

	stored := storedSMTPConfig{
		Host: cfg.Host, Port: cfg.Port, Username: cfg.Username,
		EncryptedPassword: encryptedPassword, FromAddress: cfg.FromAddress, TLSMode: cfg.TLSMode,
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encoding smtp config: %w", err)
	}
	return SetSetting(sqldb, smtpConfigSettingKey, string(data))
}

// SeedSMTPConfigFromEnv saves cfg as the server's SMTP configuration only
// if none has ever been saved -- lets a fresh Docker deployment start
// with working outgoing mail straight from docker-compose env vars,
// while leaving an admin's later edits via Settings as the permanent
// source of truth from then on (a redeploy never silently reverts
// them). No-op if cfg.Host is empty (nothing to seed).
func SeedSMTPConfigFromEnv(sqldb *sql.DB, cfg SMTPConfig) error {
	if cfg.Host == "" {
		return nil
	}
	_, found, err := GetSMTPConfig(sqldb)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return SaveSMTPConfig(sqldb, cfg)
}

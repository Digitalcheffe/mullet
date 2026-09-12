package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const webhookConfigSettingKey = "webhook_config"

// DefaultWebhookPayloadTemplate is what a fresh install sends until an
// admin customizes it. {event}/{subject}/{message}/{timestamp} are
// substituted at send time (see internal/notify's renderPayloadTemplate)
// -- each is JSON-string-escaped before substitution, so the template
// supplies the surrounding quotes rather than the substituted value.
const DefaultWebhookPayloadTemplate = `{
  "event": "{event}",
  "subject": "{subject}",
  "message": "{message}",
  "timestamp": "{timestamp}"
}`

// WebhookConfig is the server's outgoing webhook configuration (issue
// #112). Secret is always the plaintext value here -- GetWebhookConfig
// decrypts it before returning, and SaveWebhookConfig encrypts it
// before storing, mirroring SMTPConfig's own password handling.
type WebhookConfig struct {
	URL             string
	Secret          string
	PayloadTemplate string
}

// storedWebhookConfig is WebhookConfig's on-disk shape -- the secret is
// encrypted at rest the same way the SMTP password is. An empty
// PayloadTemplate means "use the default", rather than "keep whatever
// was there before" -- unlike Secret, this field is always visible in
// the Settings textarea, so there's no write-only value to preserve.
type storedWebhookConfig struct {
	URL             string `json:"url"`
	EncryptedSecret string `json:"encrypted_secret"`
	PayloadTemplate string `json:"payload_template"`
}

// GetWebhookConfig returns the server's saved webhook configuration, or
// found=false if none has ever been saved. PayloadTemplate is always
// populated -- DefaultWebhookPayloadTemplate if nothing custom was saved.
func GetWebhookConfig(sqldb *sql.DB) (WebhookConfig, bool, error) {
	raw, found, err := GetSetting(sqldb, webhookConfigSettingKey)
	if err != nil {
		return WebhookConfig{}, false, err
	}
	if !found {
		return WebhookConfig{PayloadTemplate: DefaultWebhookPayloadTemplate}, false, nil
	}

	var stored storedWebhookConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return WebhookConfig{}, false, fmt.Errorf("decoding stored webhook config: %w", err)
	}

	secret := ""
	if stored.EncryptedSecret != "" {
		key, err := loadOrCreateWebhookEncryptionKey(sqldb)
		if err != nil {
			return WebhookConfig{}, false, err
		}
		secret, err = decryptString(key, stored.EncryptedSecret)
		if err != nil {
			return WebhookConfig{}, false, err
		}
	}

	payloadTemplate := stored.PayloadTemplate
	if payloadTemplate == "" {
		payloadTemplate = DefaultWebhookPayloadTemplate
	}

	return WebhookConfig{URL: stored.URL, Secret: secret, PayloadTemplate: payloadTemplate}, true, nil
}

// SaveWebhookConfig persists cfg, encrypting the secret before storing
// it. An empty cfg.Secret keeps whatever secret was already saved --
// the frontend never receives the existing secret back to resubmit, so
// this can't mean "clear it" (same reasoning as SaveSMTPConfig). An
// empty cfg.PayloadTemplate, unlike Secret, really does mean "reset to
// the default" -- GetWebhookConfig fills it back in from empty.
func SaveWebhookConfig(sqldb *sql.DB, cfg WebhookConfig) error {
	key, err := loadOrCreateWebhookEncryptionKey(sqldb)
	if err != nil {
		return err
	}

	encryptedSecret := ""
	if cfg.Secret != "" {
		encryptedSecret, err = encryptString(key, cfg.Secret)
		if err != nil {
			return err
		}
	} else if existingRaw, found, err := GetSetting(sqldb, webhookConfigSettingKey); err != nil {
		return err
	} else if found {
		var existing storedWebhookConfig
		if err := json.Unmarshal([]byte(existingRaw), &existing); err == nil {
			encryptedSecret = existing.EncryptedSecret
		}
	}

	stored := storedWebhookConfig{URL: cfg.URL, EncryptedSecret: encryptedSecret, PayloadTemplate: cfg.PayloadTemplate}
	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encoding webhook config: %w", err)
	}
	return SetSetting(sqldb, webhookConfigSettingKey, string(data))
}

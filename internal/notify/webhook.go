package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// ErrWebhookNotConfigured is returned by TestWebhook when no webhook URL
// has been saved -- unlike the per-event deliverWebhook path, an
// explicit test action should say so rather than silently doing nothing.
var ErrWebhookNotConfigured = errors.New("webhook not configured")

// ErrInvalidPayloadTemplate is returned by ValidatePayloadTemplate (and
// surfaced as a 400 by handlePutWebhookConfig) when a template doesn't
// render to valid JSON.
var ErrInvalidPayloadTemplate = errors.New("payload template must render to valid JSON")

var webhookHTTPClient = &http.Client{Timeout: 10 * time.Second}

// jsonEscape returns s escaped for splicing into a JSON string literal
// -- the contents only, no surrounding quotes, since a payload template
// supplies those itself (e.g. `"message": "{message}"`). json.Marshal
// already produces a correctly quoted-and-escaped JSON string for any
// Go string, so this just strips the quotes it adds.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// RenderPayloadTemplate substitutes {event}, {subject}, {message}, and
// {timestamp} into tmpl, JSON-escaping each value first. It's exported
// so the Settings handler can validate an admin-edited template before
// saving it (see ValidatePayloadTemplate) using the same substitution
// this package uses to actually send one.
func RenderPayloadTemplate(tmpl, event, subject, message string, timestamp time.Time) string {
	r := strings.NewReplacer(
		"{event}", jsonEscape(event),
		"{subject}", jsonEscape(subject),
		"{message}", jsonEscape(message),
		"{timestamp}", jsonEscape(timestamp.UTC().Format(time.RFC3339)),
	)
	return r.Replace(tmpl)
}

// ValidatePayloadTemplate reports whether tmpl renders to valid JSON
// once its variables are substituted with representative sample values.
// A template that's invalid JSON before substitution can still be valid
// after (e.g. a bare `{message}` with no surrounding quotes only
// becomes valid once escaped text fills it in) -- and one that looks
// valid before substitution can break after, if a variable sits outside
// any string literal -- so this always renders first, the same way an
// actual delivery would, rather than checking tmpl on its own.
func ValidatePayloadTemplate(tmpl string) error {
	sample := RenderPayloadTemplate(tmpl, "test", "Test subject", "Test message", time.Now())
	if !json.Valid([]byte(sample)) {
		return ErrInvalidPayloadTemplate
	}
	return nil
}

// deliverWebhook POSTs eventKey/subject/body to the configured webhook
// URL. Like broadcastEmail, it no-ops cleanly (returns nil, sending
// nothing) when no webhook URL is saved, so callers can fire a
// notification unconditionally.
func deliverWebhook(sqldb *sql.DB, eventKey, subject, body string) error {
	cfg, found, err := db.GetWebhookConfig(sqldb)
	if err != nil {
		return fmt.Errorf("loading webhook config: %w", err)
	}
	if !found || cfg.URL == "" {
		return nil
	}
	return sendWebhook(cfg, eventKey, subject, body)
}

// TestWebhook sends a one-off test payload to the currently saved
// webhook URL, so an admin can confirm it actually works before relying
// on it. Returns ErrWebhookNotConfigured if no URL has been saved.
func TestWebhook(sqldb *sql.DB) error {
	cfg, found, err := db.GetWebhookConfig(sqldb)
	if err != nil {
		return fmt.Errorf("loading webhook config: %w", err)
	}
	if !found || cfg.URL == "" {
		return ErrWebhookNotConfigured
	}
	return sendWebhook(cfg, "test", "Mullet test webhook",
		"This is a test webhook delivery from your Mullet server. If you received this, your webhook URL (and secret, if set) are configured correctly.")
}

func sendWebhook(cfg db.WebhookConfig, eventKey, subject, body string) error {
	tmpl := cfg.PayloadTemplate
	if tmpl == "" {
		tmpl = db.DefaultWebhookPayloadTemplate
	}
	data := []byte(RenderPayloadTemplate(tmpl, eventKey, subject, body, time.Now()))

	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write(data)
		req.Header.Set("X-Mullet-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("delivering webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

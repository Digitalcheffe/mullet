package db

import (
	"strings"
	"testing"
)

func TestGetWebhookConfigNotYetSaved(t *testing.T) {
	sqldb := newTestDB(t)
	cfg, found, err := GetWebhookConfig(sqldb)
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if found {
		t.Error("GetWebhookConfig found=true before anything was ever saved")
	}
	if cfg.PayloadTemplate != DefaultWebhookPayloadTemplate {
		t.Errorf("PayloadTemplate (unsaved) = %q, want the default", cfg.PayloadTemplate)
	}
}

func TestWebhookConfigRoundTripEncryptsSecret(t *testing.T) {
	sqldb := newTestDB(t)

	err := SaveWebhookConfig(sqldb, WebhookConfig{
		URL: "https://example.com/hook", Secret: "s3cret-signing-key", PayloadTemplate: `{"e":"{event}"}`,
	})
	if err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}

	got, found, err := GetWebhookConfig(sqldb)
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if !found {
		t.Fatal("GetWebhookConfig found=false after saving")
	}
	if got.URL != "https://example.com/hook" || got.PayloadTemplate != `{"e":"{event}"}` {
		t.Errorf("GetWebhookConfig = %+v, unexpected values", got)
	}
	if got.Secret != "s3cret-signing-key" {
		t.Errorf("GetWebhookConfig.Secret = %q, want the original plaintext back out", got.Secret)
	}

	raw, _, err := GetSetting(sqldb, webhookConfigSettingKey)
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if strings.Contains(raw, "s3cret-signing-key") {
		t.Error("stored webhook_config contains the plaintext secret -- it must be encrypted")
	}
}

func TestSaveWebhookConfigEmptySecretKeepsExisting(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SaveWebhookConfig(sqldb, WebhookConfig{URL: "https://example.com/hook", Secret: "original-secret"}); err != nil {
		t.Fatalf("initial SaveWebhookConfig: %v", err)
	}
	if err := SaveWebhookConfig(sqldb, WebhookConfig{URL: "https://example.com/hook2", Secret: ""}); err != nil {
		t.Fatalf("second SaveWebhookConfig: %v", err)
	}

	got, _, err := GetWebhookConfig(sqldb)
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if got.URL != "https://example.com/hook2" {
		t.Errorf("URL = %q, want the updated value", got.URL)
	}
	if got.Secret != "original-secret" {
		t.Errorf("Secret = %q, want the original secret preserved", got.Secret)
	}
}

func TestSaveWebhookConfigEmptyPayloadTemplateResetsToDefault(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SaveWebhookConfig(sqldb, WebhookConfig{URL: "https://example.com/hook", PayloadTemplate: `{"custom":"{message}"}`}); err != nil {
		t.Fatalf("initial SaveWebhookConfig: %v", err)
	}
	if err := SaveWebhookConfig(sqldb, WebhookConfig{URL: "https://example.com/hook", PayloadTemplate: ""}); err != nil {
		t.Fatalf("second SaveWebhookConfig: %v", err)
	}

	got, _, err := GetWebhookConfig(sqldb)
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if got.PayloadTemplate != DefaultWebhookPayloadTemplate {
		t.Errorf("PayloadTemplate = %q, want reset to the default", got.PayloadTemplate)
	}
}

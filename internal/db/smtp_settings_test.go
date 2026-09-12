package db

import (
	"strings"
	"testing"
)

func TestGetSMTPConfigNotYetSaved(t *testing.T) {
	sqldb := newTestDB(t)
	_, found, err := GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if found {
		t.Error("GetSMTPConfig found=true before anything was ever saved")
	}
}

func TestSMTPConfigRoundTripEncryptsPassword(t *testing.T) {
	sqldb := newTestDB(t)

	err := SaveSMTPConfig(sqldb, SMTPConfig{
		Host: "smtp.example.com", Port: 587, Username: "mullet",
		Password: "s3cret-app-password", FromAddress: "mullet@example.com", TLSMode: SMTPTLSStartTLS,
	})
	if err != nil {
		t.Fatalf("SaveSMTPConfig: %v", err)
	}

	got, found, err := GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if !found {
		t.Fatal("GetSMTPConfig found=false after saving")
	}
	if got.Host != "smtp.example.com" || got.Port != 587 || got.Username != "mullet" ||
		got.FromAddress != "mullet@example.com" || got.TLSMode != SMTPTLSStartTLS {
		t.Errorf("GetSMTPConfig = %+v, unexpected values", got)
	}
	if got.Password != "s3cret-app-password" {
		t.Errorf("GetSMTPConfig.Password = %q, want the original plaintext back out", got.Password)
	}

	// The stored form must not contain the plaintext password anywhere
	// -- it should only ever exist encrypted at rest.
	raw, _, err := GetSetting(sqldb, smtpConfigSettingKey)
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if strings.Contains(raw, "s3cret-app-password") {
		t.Error("stored smtp_config contains the plaintext password -- it must be encrypted")
	}
}

func TestSaveSMTPConfigEmptyPasswordKeepsExisting(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SaveSMTPConfig(sqldb, SMTPConfig{
		Host: "smtp.example.com", Port: 587, Username: "mullet",
		Password: "original-password", FromAddress: "mullet@example.com", TLSMode: SMTPTLSStartTLS,
	}); err != nil {
		t.Fatalf("initial SaveSMTPConfig: %v", err)
	}

	// Update host only, leaving Password empty -- should not clear the
	// previously saved password.
	if err := SaveSMTPConfig(sqldb, SMTPConfig{
		Host: "smtp2.example.com", Port: 465, Username: "mullet",
		Password: "", FromAddress: "mullet@example.com", TLSMode: SMTPTLSTLS,
	}); err != nil {
		t.Fatalf("second SaveSMTPConfig: %v", err)
	}

	got, _, err := GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if got.Host != "smtp2.example.com" {
		t.Errorf("Host = %q, want the updated value", got.Host)
	}
	if got.Password != "original-password" {
		t.Errorf("Password = %q, want the original password preserved", got.Password)
	}
}

func TestSeedSMTPConfigFromEnvOnlySeedsWhenUnset(t *testing.T) {
	sqldb := newTestDB(t)

	envCfg := SMTPConfig{
		Host: "smtp.example.com", Port: 587, Username: "mullet",
		Password: "env-password", FromAddress: "mullet@example.com", TLSMode: SMTPTLSStartTLS,
	}
	if err := SeedSMTPConfigFromEnv(sqldb, envCfg); err != nil {
		t.Fatalf("SeedSMTPConfigFromEnv: %v", err)
	}
	got, found, err := GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if !found || got.Host != "smtp.example.com" {
		t.Fatalf("GetSMTPConfig after seed = (found=%v, %+v), want the seeded config", found, got)
	}

	// An admin's later edit via Settings must not be overwritten by a
	// later seed attempt (e.g. the next container restart/redeploy).
	if err := SaveSMTPConfig(sqldb, SMTPConfig{
		Host: "admin-changed.example.com", Port: 465, Username: "someone-else",
		Password: "admin-password", FromAddress: "someone@example.com", TLSMode: SMTPTLSTLS,
	}); err != nil {
		t.Fatalf("SaveSMTPConfig: %v", err)
	}
	if err := SeedSMTPConfigFromEnv(sqldb, envCfg); err != nil {
		t.Fatalf("SeedSMTPConfigFromEnv (second attempt): %v", err)
	}
	got, _, err = GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if got.Host != "admin-changed.example.com" {
		t.Errorf("Host after second seed attempt = %q, want the admin's own value preserved", got.Host)
	}
}

func TestSeedSMTPConfigFromEnvNoopWhenHostEmpty(t *testing.T) {
	sqldb := newTestDB(t)

	if err := SeedSMTPConfigFromEnv(sqldb, SMTPConfig{Host: ""}); err != nil {
		t.Fatalf("SeedSMTPConfigFromEnv: %v", err)
	}
	_, found, err := GetSMTPConfig(sqldb)
	if err != nil {
		t.Fatalf("GetSMTPConfig: %v", err)
	}
	if found {
		t.Error("GetSMTPConfig found=true after a no-op seed with an empty host")
	}
}


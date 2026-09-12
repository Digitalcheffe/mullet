package db

import (
	"errors"
	"strings"
	"testing"
)

func TestTOTPEnrollmentLifecycle(t *testing.T) {
	sqldb := newTestDB(t)
	user, err := CreateUser(sqldb, "alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, err := GetTOTPSecret(sqldb, user.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTOTPSecret (before enrollment) = %v, want ErrNotFound", err)
	}

	if err := StartTOTPEnrollment(sqldb, user.ID, "SECRETVALUE"); err != nil {
		t.Fatalf("StartTOTPEnrollment: %v", err)
	}

	got, err := GetTOTPSecret(sqldb, user.ID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got != "SECRETVALUE" {
		t.Errorf("GetTOTPSecret = %q, want SECRETVALUE", got)
	}

	// Not enabled until confirmed.
	reloaded, err := GetUser(sqldb, user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if reloaded.TOTPEnabled {
		t.Error("TOTPEnabled = true before confirmation")
	}

	if err := ConfirmTOTPEnrollment(sqldb, user.ID); err != nil {
		t.Fatalf("ConfirmTOTPEnrollment: %v", err)
	}
	reloaded, _ = GetUser(sqldb, user.ID)
	if !reloaded.TOTPEnabled {
		t.Error("TOTPEnabled = false after confirmation")
	}

	if err := DisableTOTP(sqldb, user.ID); err != nil {
		t.Fatalf("DisableTOTP: %v", err)
	}
	reloaded, _ = GetUser(sqldb, user.ID)
	if reloaded.TOTPEnabled {
		t.Error("TOTPEnabled = true after DisableTOTP")
	}
	if _, err := GetTOTPSecret(sqldb, user.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTOTPSecret (after disable) = %v, want ErrNotFound", err)
	}
}

func TestTOTPSecretIsEncryptedAtRest(t *testing.T) {
	sqldb := newTestDB(t)
	user, err := CreateUser(sqldb, "alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := StartTOTPEnrollment(sqldb, user.ID, "PLAINTEXT-SECRET-VALUE"); err != nil {
		t.Fatalf("StartTOTPEnrollment: %v", err)
	}

	var stored string
	if err := sqldb.QueryRow(`SELECT totp_secret FROM users WHERE id = ?`, user.ID).Scan(&stored); err != nil {
		t.Fatalf("querying raw column: %v", err)
	}
	if strings.Contains(stored, "PLAINTEXT-SECRET-VALUE") {
		t.Error("stored totp_secret contains the plaintext secret -- it must be encrypted")
	}
}

func TestBackupCodesRoundTrip(t *testing.T) {
	sqldb := newTestDB(t)
	user, err := CreateUser(sqldb, "alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	codes, err := GenerateBackupCodes(sqldb, user.ID)
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) != backupCodeCount {
		t.Fatalf("GenerateBackupCodes returned %d codes, want %d", len(codes), backupCodeCount)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate backup code generated: %q", c)
		}
		seen[c] = true
	}

	if remaining, err := CountRemainingBackupCodes(sqldb, user.ID); err != nil || remaining != backupCodeCount {
		t.Fatalf("CountRemainingBackupCodes = %d, %v, want %d, nil", remaining, err, backupCodeCount)
	}

	ok, err := ConsumeBackupCode(sqldb, user.ID, codes[0])
	if err != nil || !ok {
		t.Fatalf("ConsumeBackupCode (valid, first use) = %v, %v, want true, nil", ok, err)
	}

	// The same code can't be used twice.
	ok, err = ConsumeBackupCode(sqldb, user.ID, codes[0])
	if err != nil || ok {
		t.Fatalf("ConsumeBackupCode (reuse) = %v, %v, want false, nil", ok, err)
	}

	ok, err = ConsumeBackupCode(sqldb, user.ID, "NOPE-NOPE")
	if err != nil || ok {
		t.Fatalf("ConsumeBackupCode (unknown code) = %v, %v, want false, nil", ok, err)
	}

	if remaining, err := CountRemainingBackupCodes(sqldb, user.ID); err != nil || remaining != backupCodeCount-1 {
		t.Fatalf("CountRemainingBackupCodes after one use = %d, %v, want %d, nil", remaining, err, backupCodeCount-1)
	}
}

func TestGenerateBackupCodesReplacesOldSet(t *testing.T) {
	sqldb := newTestDB(t)
	user, err := CreateUser(sqldb, "alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	first, err := GenerateBackupCodes(sqldb, user.ID)
	if err != nil {
		t.Fatalf("GenerateBackupCodes (first): %v", err)
	}
	if _, err := GenerateBackupCodes(sqldb, user.ID); err != nil {
		t.Fatalf("GenerateBackupCodes (second): %v", err)
	}

	ok, err := ConsumeBackupCode(sqldb, user.ID, first[0])
	if err != nil || ok {
		t.Errorf("ConsumeBackupCode (from replaced set) = %v, %v, want false, nil", ok, err)
	}
	if remaining, err := CountRemainingBackupCodes(sqldb, user.ID); err != nil || remaining != backupCodeCount {
		t.Errorf("CountRemainingBackupCodes after regeneration = %d, %v, want %d, nil", remaining, err, backupCodeCount)
	}
}

func TestDisableTOTPDeletesBackupCodes(t *testing.T) {
	sqldb := newTestDB(t)
	user, err := CreateUser(sqldb, "alice", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := GenerateBackupCodes(sqldb, user.ID); err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if err := DisableTOTP(sqldb, user.ID); err != nil {
		t.Fatalf("DisableTOTP: %v", err)
	}
	if remaining, err := CountRemainingBackupCodes(sqldb, user.ID); err != nil || remaining != 0 {
		t.Errorf("CountRemainingBackupCodes after disable = %d, %v, want 0, nil", remaining, err)
	}
}

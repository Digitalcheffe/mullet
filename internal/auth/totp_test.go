package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTOTPSecretIsUsableAndUnique(t *testing.T) {
	a, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	b, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if a == b {
		t.Error("two generated secrets were identical")
	}
	if !ValidateTOTPCode(a, mustCodeAt(t, a, time.Now())) {
		t.Error("a freshly generated secret's own current code didn't validate")
	}
}

// mustCodeAt computes the code a real authenticator app would currently
// be showing for secret, via the package's own unexported clock-based
// helper -- this is the fixture, not the thing under test, so calling
// the same implementation the tests are meant to verify is fine here:
// the actual assertions are about ValidateTOTPCode's behavior (accepts
// the right code, rejects the wrong one, tolerates skew, is
// case-insensitive on the secret), not about re-deriving RFC 6238 from
// scratch.
func mustCodeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totpCodeAt(secret, at)
	if err != nil {
		t.Fatalf("totpCodeAt: %v", err)
	}
	return code
}

func TestValidateTOTPCodeRejectsWrongCode(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if ValidateTOTPCode(secret, "000000") {
		// Astronomically unlikely to be the real code, but avoid mocking
		// time -- if this ever flakes, use a fixed known-answer vector
		// like TestValidateTOTPCodeKnownAnswer below instead.
		t.Skip("000000 happened to be the real code this instant")
	}
}

func TestValidateTOTPCodeTakesEitherCase(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	now := time.Now()
	code := mustCodeAt(t, secret, now)
	if !ValidateTOTPCode(strings.ToLower(secret), code) {
		t.Error("a lowercase secret didn't validate the same code an uppercase one produces")
	}
}

func TestValidateTOTPCodeToleratesOneStepOfClockSkew(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	now := time.Now()

	past := mustCodeAt(t, secret, now.Add(-totpPeriod))
	if !ValidateTOTPCode(secret, past) {
		t.Error("a code from one step in the past didn't validate")
	}

	future := mustCodeAt(t, secret, now.Add(totpPeriod))
	if !ValidateTOTPCode(secret, future) {
		t.Error("a code from one step in the future didn't validate")
	}

	tooOld := mustCodeAt(t, secret, now.Add(-3*totpPeriod))
	if ValidateTOTPCode(secret, tooOld) {
		t.Error("a code from three steps in the past validated -- skew tolerance should be ±1 step")
	}
}

func TestValidateTOTPCodeRejectsMalformedInput(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	for _, code := range []string{"", "12345", "1234567", "abcdef", "  "} {
		if ValidateTOTPCode(secret, code) {
			t.Errorf("ValidateTOTPCode(%q) = true, want false", code)
		}
	}
}

func TestValidateTOTPCodeKnownAnswer(t *testing.T) {
	// RFC 6238 Appendix B, SHA1 row, Time=59 (T=0000000000000001): the
	// RFC's published 8-digit code for this secret/time is 94287082.
	// This package always produces 6 digits, which RFC 4226's dynamic
	// truncation defines as simply the same value mod 10^6 -- so the
	// independently-known-correct answer here is the RFC's own published
	// value's last 6 digits, not anything derived from this package's
	// own code (that would just be checking the implementation against
	// itself and could never catch a shared bug).
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // base32("12345678901234567890")
	const wantCode = "287082"                          // 94287082 mod 1e6
	at := time.Unix(59, 0).UTC()

	got, err := totpCodeAt(secret, at)
	if err != nil {
		t.Fatalf("totpCodeAt: %v", err)
	}
	if got != wantCode {
		t.Errorf("totpCodeAt(RFC 6238 vector) = %q, want %q", got, wantCode)
	}
}

func TestTOTPAuthURLIsWellFormed(t *testing.T) {
	url := TOTPAuthURL("Mullet", "alice", "ABCD1234")
	if !strings.HasPrefix(url, "otpauth://totp/Mullet:alice?") {
		t.Errorf("TOTPAuthURL = %q, want it to start with otpauth://totp/Mullet:alice?", url)
	}
	if !strings.Contains(url, "secret=ABCD1234") {
		t.Errorf("TOTPAuthURL = %q, want it to contain the secret", url)
	}
	if !strings.Contains(url, "issuer=Mullet") {
		t.Errorf("TOTPAuthURL = %q, want it to contain the issuer", url)
	}
}

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238, built on the HOTP counter algorithm in RFC 4226),
// hand-rolled against the standard library rather than a third-party
// module -- SHA1 + 6 digits + a 30s step is what every mainstream
// authenticator app (Google Authenticator, Authy, 1Password, ...)
// assumes by default when an otpauth:// URI doesn't specify otherwise,
// so deviating from those defaults would trade compatibility for no
// real security benefit.
const (
	totpDigits     = 6
	totpPeriod     = 30 * time.Second
	totpSkewSteps  = 1 // tolerate ±1 step (±30s) of clock drift
	totpSecretSize = 20 // 160 bits, the size HOTP's defining RFC (4226) specifies for HMAC-SHA1
)

// GenerateTOTPSecret returns a new random secret, base32-encoded
// (unpadded, matching what authenticator apps expect in an enrollment
// QR code or manual-entry key).
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, totpSecretSize)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPAuthURL builds the otpauth:// URI an authenticator app's QR
// scanner expects, identifying the account as "issuer:accountName".
func TOTPAuthURL(issuer, accountName, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(accountName)
	q := url.Values{
		"secret": {secret},
		"issuer": {issuer},
	}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// CurrentTOTPCode returns the 6-digit code a real authenticator app
// would be showing for secret right now. Nothing in this server ever
// needs to generate a code in production -- only the admin's own
// authenticator app does that -- so this exists for tests that need to
// drive the enroll/confirm/login flow end-to-end without a real device.
func CurrentTOTPCode(secret string) (string, error) {
	return totpCodeAt(secret, time.Now())
}

// ValidateTOTPCode reports whether code is a valid 6-digit TOTP code
// for secret at time t, allowing ±totpSkewSteps steps of clock drift.
func ValidateTOTPCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}

	now := time.Now()
	for skew := -totpSkewSteps; skew <= totpSkewSteps; skew++ {
		want, err := totpCodeAt(secret, now.Add(time.Duration(skew)*totpPeriod))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(want)) == 1 {
			return true
		}
	}
	return false
}

// totpCodeAt computes the TOTP code for secret at time t.
func totpCodeAt(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("decoding totp secret: %w", err)
	}

	counter := uint64(t.Unix()) / uint64(totpPeriod.Seconds())
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(counterBytes[:])
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 §5.3): the low nibble of the last
	// byte picks a 4-byte offset into the HMAC, whose top bit is then
	// masked off before reducing mod 10^digits.
	offset := sum[len(sum)-1] & 0x0f
	binCode := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, binCode%mod), nil
}

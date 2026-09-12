package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
)

type mfaVerifyRequest struct {
	PendingToken string `json:"pending_token"`
	// Code is either a 6-digit TOTP code or one of the account's backup
	// codes (formatted XXXX-XXXX) -- checked in that order.
	Code string `json:"code"`
}

// handleMFAVerify exchanges a pending-MFA token (issued by handleLogin
// once TOTP is required) plus a valid code for a real session token.
// Public and unauthenticated by necessity, same reasoning as login
// itself: the caller has no session yet, only the pending token.
func handleMFAVerify(sqldb *sql.DB, jwtSecret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mfaVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.PendingToken == "" || req.Code == "" {
			http.Error(w, "pending_token and code are required", http.StatusBadRequest)
			return
		}

		claims, err := auth.ParseToken(jwtSecret, req.PendingToken)
		if err != nil || !claims.Pending {
			http.Error(w, "invalid or expired login -- please sign in again", http.StatusUnauthorized)
			return
		}

		secret, err := db.GetTOTPSecret(sqldb, claims.UserID)
		if errors.Is(err, db.ErrNotFound) {
			// TOTP was disabled between login and this call -- treat as
			// invalid rather than granting a session on a stale
			// assumption.
			http.Error(w, "invalid code", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		valid := auth.ValidateTOTPCode(secret, req.Code)
		if !valid {
			valid, err = db.ConsumeBackupCode(sqldb, claims.UserID, req.Code)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
		if !valid {
			http.Error(w, "invalid code", http.StatusUnauthorized)
			return
		}

		token, err := auth.IssueToken(jwtSecret, claims.UserID, claims.Username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: token})
	}
}

type totpStatusResponse struct {
	Enabled              bool `json:"enabled"`
	BackupCodesRemaining int  `json:"backup_codes_remaining"`
}

// handleGetTOTPStatus reports whether the authenticated user has TOTP
// enabled, and how many backup codes they have left.
func handleGetTOTPStatus(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := db.GetUser(sqldb, claims.UserID)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		remaining := 0
		if user.TOTPEnabled {
			remaining, err = db.CountRemainingBackupCodes(sqldb, claims.UserID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(totpStatusResponse{Enabled: user.TOTPEnabled, BackupCodesRemaining: remaining})
	}
}

type totpEnrollResponse struct {
	Secret  string `json:"secret"`
	AuthURL string `json:"auth_url"`
}

// handleEnrollTOTP starts enrollment: generates a new secret, stages it
// unconfirmed, and returns it (plus a scannable otpauth:// URL) for the
// frontend to show as a QR code. Refuses if TOTP is already enabled --
// disable it first to switch authenticator apps, rather than silently
// replacing a working confirmed secret with an unconfirmed one.
func handleEnrollTOTP(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := db.GetUser(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if user.TOTPEnabled {
			http.Error(w, "two-factor authentication is already enabled -- disable it first to re-enroll", http.StatusConflict)
			return
		}

		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := db.StartTOTPEnrollment(sqldb, claims.UserID, secret); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(totpEnrollResponse{
			Secret:  secret,
			AuthURL: auth.TOTPAuthURL("Mullet", user.Username, secret),
		})
	}
}

type totpConfirmRequest struct {
	Code string `json:"code"`
}

type totpConfirmResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

// handleConfirmTOTP completes enrollment: verifies one code against the
// secret handleEnrollTOTP staged, and only then flips totp_enabled on
// and mints backup codes -- an enrollment is never active until proven
// to actually work with the admin's authenticator app.
func handleConfirmTOTP(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req totpConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		secret, err := db.GetTOTPSecret(sqldb, claims.UserID)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "start enrollment first", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !auth.ValidateTOTPCode(secret, req.Code) {
			http.Error(w, "that code didn't match -- check your authenticator app and try again", http.StatusBadRequest)
			return
		}

		if err := db.ConfirmTOTPEnrollment(sqldb, claims.UserID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		codes, err := db.GenerateBackupCodes(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(totpConfirmResponse{BackupCodes: codes})
	}
}

type totpPasswordConfirmRequest struct {
	Password string `json:"password"`
}

// handleDisableTOTP turns off TOTP for the authenticated user, requiring
// their current password as confirmation -- this is a security-lowering
// action, so it shouldn't be reachable by, say, a stolen unlocked
// session alone with no further proof of identity.
func handleDisableTOTP(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req totpPasswordConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		user, err := db.GetUser(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			// 403, not 401 -- see account_handlers.go's handleUpdateAccountPassword
			// for why: useApiFetch force-logs-out on any 401, which a wrong
			// password here shouldn't trigger, since the session itself is fine.
			http.Error(w, "current password is incorrect", http.StatusForbidden)
			return
		}

		if err := db.DisableTOTP(sqldb, claims.UserID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleRegenerateBackupCodes replaces the authenticated user's backup
// codes, requiring their current password for the same reason
// handleDisableTOTP does -- the old codes stop working the moment this
// succeeds.
func handleRegenerateBackupCodes(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req totpPasswordConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		user, err := db.GetUser(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			// 403, not 401 -- see account_handlers.go's handleUpdateAccountPassword
			// for why: useApiFetch force-logs-out on any 401, which a wrong
			// password here shouldn't trigger, since the session itself is fine.
			http.Error(w, "current password is incorrect", http.StatusForbidden)
			return
		}
		if !user.TOTPEnabled {
			http.Error(w, "two-factor authentication isn't enabled", http.StatusBadRequest)
			return
		}

		codes, err := db.GenerateBackupCodes(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(totpConfirmResponse{BackupCodes: codes})
	}
}

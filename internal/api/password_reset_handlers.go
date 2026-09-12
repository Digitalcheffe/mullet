package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/email"
	"github.com/Digitalcheffe/mullet/internal/notify"
)

type forgotPasswordRequest struct {
	Username string `json:"username"`
}

// handleForgotPassword starts a password reset: if SMTP is configured
// and the named account has an email on file, emails it a time-limited
// reset link. Public and unauthenticated by necessity -- this exists
// for someone who's already locked out.
//
// Deliberately doesn't hide whether SMTP is configured or whether a
// found account has an email set (issue #78 explicitly asks for clear
// failure messages here, not a silent no-op) -- a self-hosted server
// with a handful of trusted admins isn't the same threat model as a
// public SaaS signup form, where those messages would be a user-
// enumeration concern. It does still return the same response for "no
// such username" as for "email sent successfully," rather than
// confirming or denying a specific username exists.
func handleForgotPassword(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req forgotPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Username == "" {
			http.Error(w, "username is required", http.StatusBadRequest)
			return
		}

		cfg, found, err := db.GetSMTPConfig(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !found || cfg.Host == "" {
			http.Error(w, "email isn't configured on this server -- ask your administrator to set up SMTP in Settings", http.StatusServiceUnavailable)
			return
		}

		user, err := db.GetUserByUsername(sqldb, req.Username)
		if errors.Is(err, db.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if user.Email == nil || *user.Email == "" {
			http.Error(w, "this account has no email address on file -- ask your administrator to set one in Settings", http.StatusUnprocessableEntity)
			return
		}

		token, err := db.CreatePasswordResetToken(sqldb, user.ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resetLink := publicBaseURL(r) + "/admin/reset-password?token=" + url.QueryEscape(token)
		body := fmt.Sprintf(
			"A password reset was requested for your Mullet account (%s).\n\n"+
				"Reset your password: %s\n\n"+
				"This link expires in 1 hour and can only be used once. If you didn't request this, you can safely ignore this email.",
			user.Username, resetLink,
		)
		err = email.Send(email.Config{
			Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password,
			FromAddress: cfg.FromAddress, TLSMode: cfg.TLSMode,
		}, *user.Email, "Reset your Mullet password", body)
		if err != nil {
			http.Error(w, "failed to send reset email: "+err.Error(), http.StatusBadGateway)
			return
		}

		if err := notify.PasswordResetRequested(sqldb, user.Username); err != nil {
			log.Printf("password reset requested for user %d but notification failed: %v", user.ID, err)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
	// DisableMFA additionally turns off TOTP (issue #114) for the
	// account being reset. Verified email ownership is treated as
	// sufficient proof of identity for this, the same as it already is
	// for the password itself -- otherwise a lost authenticator device
	// would leave an admin locked out even after successfully resetting
	// their password. Harmless to set on an account that never had TOTP
	// enabled in the first place.
	DisableMFA bool `json:"disable_mfa"`
}

// handleResetPassword completes a reset: redeems token (single-use,
// see db.ConsumePasswordResetToken) and sets the associated account's
// new password. Public and unauthenticated for the same reason
// handleForgotPassword is -- the token itself is the credential here.
func handleResetPassword(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req resetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Token == "" || len(req.NewPassword) < 8 {
			http.Error(w, "token is required and new_password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		userID, err := db.ConsumePasswordResetToken(sqldb, req.Token)
		if errors.Is(err, db.ErrResetTokenInvalid) {
			http.Error(w, "this reset link is invalid, expired, or already used -- request a new one", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := db.SetUserPasswordHash(sqldb, userID, hash); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if req.DisableMFA {
			if err := db.DisableTOTP(sqldb, userID); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		if user, err := db.GetUser(sqldb, userID); err != nil {
			log.Printf("password reset completed for user %d but looking it up for notification failed: %v", userID, err)
		} else if err := notify.PasswordResetCompleted(sqldb, user.Username); err != nil {
			log.Printf("password reset completed for user %d but notification failed: %v", userID, err)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// publicBaseURL derives the origin to build a password-reset link
// against from the incoming request itself, rather than a separate
// "canonical URL" setting -- this app has no existing concept of one,
// and a self-hosted LAN deployment's own Host header is already the
// right answer. X-Forwarded-Proto covers the common reverse-proxy case
// where the server itself only ever sees plain HTTP.
func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

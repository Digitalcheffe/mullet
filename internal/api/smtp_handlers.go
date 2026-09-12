package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/email"
)

var validTLSModes = map[string]bool{
	db.SMTPTLSNone:     true,
	db.SMTPTLSStartTLS: true,
	db.SMTPTLSTLS:      true,
}

type smtpConfigResponse struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	FromAddress string `json:"from_address"`
	TLSMode     string `json:"tls_mode"`
	// HasPassword reports whether a password is currently saved, without
	// ever sending the password itself back to the browser.
	HasPassword bool `json:"has_password"`
}

// handleGetSMTPConfig returns the saved SMTP configuration, or the zero
// value with every field empty if nothing has been saved yet.
func handleGetSMTPConfig(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _, err := db.GetSMTPConfig(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(smtpConfigResponse{
			Host: cfg.Host, Port: cfg.Port, Username: cfg.Username,
			FromAddress: cfg.FromAddress, TLSMode: cfg.TLSMode, HasPassword: cfg.Password != "",
		})
	}
}

type smtpConfigRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Username string `json:"username"`
	// Password left empty keeps whatever password is already saved --
	// the frontend never receives the real password back to resubmit,
	// so there's no other way to mean "leave it as-is" vs. "clear it."
	// Clearing a saved password isn't supported directly; replace the
	// whole config (or blank the username too) if that's really needed.
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	TLSMode     string `json:"tls_mode"`
}

// handlePutSMTPConfig saves the SMTP configuration used for password
// reset emails and the test-email button.
func handlePutSMTPConfig(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req smtpConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Host == "" || req.Port <= 0 || req.Port > 65535 || req.FromAddress == "" {
			http.Error(w, "host, a valid port, and from_address are required", http.StatusBadRequest)
			return
		}
		if req.TLSMode == "" {
			req.TLSMode = db.SMTPTLSStartTLS
		}
		if !validTLSModes[req.TLSMode] {
			http.Error(w, "tls_mode must be one of: none, starttls, tls", http.StatusBadRequest)
			return
		}

		err := db.SaveSMTPConfig(sqldb, db.SMTPConfig{
			Host: req.Host, Port: req.Port, Username: req.Username,
			Password: req.Password, FromAddress: req.FromAddress, TLSMode: req.TLSMode,
		})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type testEmailRequest struct {
	// To left empty falls back to the requesting admin's own email (see
	// db.GetUser) -- lets "send a test email" be a single click for the
	// common case of testing delivery to yourself.
	To string `json:"to"`
}

// handleTestSMTP sends a one-off test message using the currently saved
// SMTP configuration, so an admin can confirm it actually works before
// relying on it for password reset.
func handleTestSMTP(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, found, err := db.GetSMTPConfig(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !found || cfg.Host == "" {
			http.Error(w, "SMTP isn't configured yet -- save your settings first", http.StatusBadRequest)
			return
		}

		var req testEmailRequest
		json.NewDecoder(r.Body).Decode(&req) // empty/absent body is fine -- To just stays ""

		to := req.To
		if to == "" {
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
			if user.Email == nil || *user.Email == "" {
				http.Error(w, "enter an address to send the test to, or set an email on your account first", http.StatusBadRequest)
				return
			}
			to = *user.Email
		}

		err = email.Send(email.Config{
			Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password,
			FromAddress: cfg.FromAddress, TLSMode: cfg.TLSMode,
		}, to, "Mullet test email", "This is a test email from your Mullet server. If you received this, SMTP is configured correctly.")
		if err != nil {
			http.Error(w, "failed to send: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
)

type updateAccountEmailRequest struct {
	// Empty clears the email -- it's optional (see users.email), only
	// needed if this account wants password-reset-via-email to work.
	Email string `json:"email"`
}

// handleUpdateAccountEmail sets or clears the authenticated user's own
// email address, used for password reset (issue #78) and as the
// default recipient for the Settings page's test-email button.
func handleUpdateAccountEmail(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req updateAccountEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(req.Email)
		// A minimal shape check, not real validation -- an email address
		// only needs to be plausible enough that mistyping it fails
		// obviously, not RFC 5322-correct.
		if email != "" && (!strings.Contains(email, "@") || strings.Contains(email, " ")) {
			http.Error(w, "that doesn't look like a valid email address", http.StatusBadRequest)
			return
		}

		if err := db.SetUserEmail(sqldb, claims.UserID, email); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type updateAccountPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleUpdateAccountPassword lets the authenticated user change their
// own password from Settings (issue #111) -- independent of #78's
// emailed reset link, which exists for someone who's already locked out
// and has no session to present here.
func handleUpdateAccountPassword(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req updateAccountPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.NewPassword) < 8 {
			http.Error(w, "new password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		user, err := db.GetUser(sqldb, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword); err != nil {
			http.Error(w, "current password is incorrect", http.StatusUnauthorized)
			return
		}

		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := db.SetUserPasswordHash(sqldb, claims.UserID, hash); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

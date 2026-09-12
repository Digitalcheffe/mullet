package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse carries either a real session Token, or (when the
// account has TOTP enabled) MFARequired plus a short-lived PendingToken
// to exchange for one via POST /api/admin/mfa/verify -- never both.
type loginResponse struct {
	Token        string `json:"token,omitempty"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
	PendingToken string `json:"pending_token,omitempty"`
}

type setupStatusResponse struct {
	Required     bool `json:"required"`
	AuthDisabled bool `json:"auth_disabled"`
}

// handleSetupStatus reports whether first-run setup (creating the initial
// admin account) still needs to happen. With authDisabled, it always
// reports setup as not required -- there's no session to gate.
func handleSetupStatus(sqldb *sql.DB, authDisabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if authDisabled {
			json.NewEncoder(w).Encode(setupStatusResponse{Required: false, AuthDisabled: true})
			return
		}

		count, err := db.CountUsers(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(setupStatusResponse{Required: count == 0})
	}
}

// handleSetup creates the initial admin account. It refuses once any user
// already exists -- setup runs exactly once.
func handleSetup(sqldb *sql.DB, jwtSecret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Username == "" || len(req.Password) < 8 {
			http.Error(w, "username is required and password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		count, err := db.CountUsers(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "setup has already been completed", http.StatusConflict)
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		result, err := sqldb.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, req.Username, hash)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		userID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		token, err := auth.IssueToken(jwtSecret, int(userID), req.Username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: token})
	}
}

// handleLogin authenticates a username/password against the users table
// and returns a signed JWT on success.
func handleLogin(sqldb *sql.DB, jwtSecret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		user, err := db.GetUserByUsername(sqldb, req.Username)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		// Password verified. If TOTP is enabled (issue #114), that's not
		// a full login yet -- issue a short-lived pending token the
		// client exchanges for a real session at /api/admin/mfa/verify,
		// rather than the session token itself.
		if user.TOTPEnabled {
			pending, err := auth.IssuePendingMFAToken(jwtSecret, user.ID, user.Username)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(loginResponse{MFARequired: true, PendingToken: pending})
			return
		}

		token, err := auth.IssueToken(jwtSecret, user.ID, user.Username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: token})
	}
}

// handleWhoAmI returns the authenticated user's identity, including
// their own email (issue #78 -- used to prefill Settings' account
// section and as the default test-email recipient).
func handleWhoAmI(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// claims.UserID has no matching row under AUTH_DISABLED (see
		// middleware.go's devClaims, UserID: 0) -- that's expected there,
		// not a real lookup failure, so it just means "no email" rather
		// than a 500.
		var userEmail *string
		if user, err := db.GetUser(sqldb, claims.UserID); err == nil {
			userEmail = user.Email
		} else if !errors.Is(err, db.ErrNotFound) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"user_id":  claims.UserID,
			"username": claims.Username,
			"email":    userEmail,
		})
	}
}

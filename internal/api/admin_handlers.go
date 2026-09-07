package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
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

		var userID int
		var passwordHash string
		err := sqldb.QueryRow(
			`SELECT id, password_hash FROM users WHERE username = ?`, req.Username,
		).Scan(&userID, &passwordHash)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := auth.VerifyPassword(passwordHash, req.Password); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := auth.IssueToken(jwtSecret, userID, req.Username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: token})
	}
}

// handleWhoAmI returns the authenticated user's identity. It exists to
// exercise requireAuth end-to-end; real admin endpoints (settings, users,
// plugins, displays, ...) land in later issues.
func handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user_id":  claims.UserID,
		"username": claims.Username,
	})
}

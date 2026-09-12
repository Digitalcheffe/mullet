package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/notify"
)

type userResponse struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u db.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, Email: u.Email, CreatedAt: u.CreatedAt}
}

// handleListUsers lists every admin account -- never the password hash.
func handleListUsers(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := db.ListUsers(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]userResponse, len(users))
		for i, u := range users {
			resp[i] = toUserResponse(u)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleCreateUser adds another admin account -- same validation as
// first-run setup (internal/api/admin_handlers.go's handleSetup), since
// this is really the same operation once more than one account is
// allowed to exist.
func handleCreateUser(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Username == "" || len(req.Password) < 8 {
			http.Error(w, "username is required and password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		user, err := db.CreateUser(sqldb, req.Username, hash)
		if errors.Is(err, db.ErrUsernameTaken) {
			http.Error(w, "that username is already taken", http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := notify.NewUser(sqldb, user.Username); err != nil {
			log.Printf("user %d created but notification failed: %v", user.ID, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toUserResponse(user))
	}
}

// handleDeleteUser removes an admin account. db.DeleteUser refuses if
// this would be the last one left -- Mullet has no account-recovery
// path, so that's enforced here rather than left to the admin to avoid.
func handleDeleteUser(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch err := db.DeleteUser(sqldb, id); {
		case errors.Is(err, db.ErrLastAdmin):
			http.Error(w, "can't remove the last remaining admin account", http.StatusConflict)
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

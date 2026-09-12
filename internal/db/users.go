package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is an admin account. Existing call sites (login, setup) still
// read/write the users table with raw SQL directly -- this file only
// covers the new operations issue #78 needs (email, password reset);
// consolidating everything onto User is left for #111 (multi-user
// management), which needs list/create/delete too.
type User struct {
	ID           int
	Username     string
	PasswordHash string
	Email        *string
	Role         string
	CreatedAt    time.Time
}

const userColumns = `id, username, password_hash, email, role, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.CreatedAt); err != nil {
		return User{}, err
	}
	return u, nil
}

// GetUser returns one user by ID, or ErrNotFound.
func GetUser(sqldb *sql.DB, id int) (User, error) {
	row := sqldb.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("getting user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByUsername returns one user by username, or ErrNotFound.
func GetUserByUsername(sqldb *sql.DB, username string) (User, error) {
	row := sqldb.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("getting user %q: %w", username, err)
	}
	return u, nil
}

// SetUserEmail updates a user's email address. An empty string clears
// it (email is nullable -- password-reset-via-email is opt-in per
// account, not required).
func SetUserEmail(sqldb *sql.DB, userID int, email string) error {
	var value any
	if email != "" {
		value = email
	}
	result, err := sqldb.Exec(`UPDATE users SET email = ? WHERE id = ?`, value, userID)
	if err != nil {
		return fmt.Errorf("updating email for user %d: %w", userID, err)
	}
	return checkRowsAffected(result, userID)
}

// SetUserPasswordHash overwrites a user's password hash -- used by the
// reset-password flow to complete a reset once its token has been
// validated and consumed.
func SetUserPasswordHash(sqldb *sql.DB, userID int, passwordHash string) error {
	result, err := sqldb.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("updating password for user %d: %w", userID, err)
	}
	return checkRowsAffected(result, userID)
}

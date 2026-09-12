package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
)

// ErrUsernameTaken is returned by CreateUser when the username's UNIQUE
// constraint rejects the insert.
var ErrUsernameTaken = errors.New("username already taken")

// ErrLastAdmin is returned by DeleteUser when removing the given account
// would leave zero admin accounts -- Mullet has no recovery path for a
// server with no way to log in, so this is refused outright rather than
// left to the caller to check first.
var ErrLastAdmin = errors.New("cannot remove the last admin account")

// User is an admin account. login and setup (internal/api/admin_handlers.go)
// still read/write the users table with raw SQL directly, since both
// predate this file and only need a password hash lookup / one-time
// insert -- not worth churning working code to route through here too.
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

// CountUsers returns how many admin accounts exist.
func CountUsers(sqldb *sql.DB) (int, error) {
	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

// ListUsers returns every admin account, oldest first.
func ListUsers(sqldb *sql.DB) ([]User, error) {
	rows, err := sqldb.Query(`SELECT ` + userColumns + ` FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateUser adds another admin account, returning ErrUsernameTaken if
// the username is already in use.
func CreateUser(sqldb *sql.DB, username, passwordHash string) (User, error) {
	result, err := sqldb.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("reading new user id: %w", err)
	}
	return GetUser(sqldb, int(id))
}

// DeleteUser removes an admin account, refusing with ErrLastAdmin if it's
// the only one left. It deletes first and counts what's left afterward,
// inside one transaction that rolls back (via defer) unless it reaches
// Commit -- checking "is this the last one" before checking "does this
// id even exist" would wrongly report ErrLastAdmin for a bogus id once
// only one real account remains. The transaction also closes the window
// for two concurrent last-account deletes to both slip past the check.
func DeleteUser(sqldb *sql.DB, id int) error {
	tx, err := sqldb.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking result for user %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}

	var remaining int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&remaining); err != nil {
		return fmt.Errorf("counting remaining users: %w", err)
	}
	if remaining == 0 {
		return ErrLastAdmin
	}
	return tx.Commit()
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint
// failure specifically (not just any constraint failure -- unlike
// displays.go's isForeignKeyViolation, this checks the full extended
// result code rather than masking down to the primary one, since a
// masked check can't tell UNIQUE apart from FOREIGN KEY).
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	const sqliteConstraintUnique = 2067 // SQLITE_CONSTRAINT_UNIQUE
	return sqliteErr.Code() == sqliteConstraintUnique
}

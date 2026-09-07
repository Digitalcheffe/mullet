package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of password, suitable for storing in
// users.password_hash.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// VerifyPassword returns nil if password matches hash, and an error
// otherwise (e.g. bcrypt.ErrMismatchedHashAndPassword).
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

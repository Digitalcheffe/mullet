// Package auth handles admin authentication: JWT issuing/verification and
// password hashing for the login flow. The generic OAuth2 flow used by
// data plugins (see issue #24) lives here too once it lands.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken covers every way a token can fail to verify: bad
// signature, malformed, expired, or wrong signing method.
var ErrInvalidToken = errors.New("invalid or expired token")

const tokenTTL = 24 * time.Hour

// Claims is the JWT payload for an authenticated admin session.
type Claims struct {
	UserID   int    `json:"uid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// IssueToken returns a signed JWT for the given user, valid for 24 hours.
func IssueToken(secret []byte, userID int, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseToken validates tokenString against secret and returns its claims.
func ParseToken(secret []byte, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

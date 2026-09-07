package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndParseToken(t *testing.T) {
	secret := []byte("test-secret")

	token, err := IssueToken(secret, 1, "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 1 || claims.Username != "admin" {
		t.Errorf("claims = %+v, want UserID=1 Username=admin", claims)
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	token, err := IssueToken([]byte("correct-secret"), 1, "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := ParseToken([]byte("wrong-secret"), token); err == nil {
		t.Error("ParseToken with wrong secret: expected error, got nil")
	}
}

func TestParseTokenExpired(t *testing.T) {
	secret := []byte("test-secret")
	claims := Claims{
		UserID:   1,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	if _, err := ParseToken(secret, token); err == nil {
		t.Error("ParseToken with expired token: expected error, got nil")
	}
}

func TestParseTokenGarbage(t *testing.T) {
	if _, err := ParseToken([]byte("test-secret"), "not-a-jwt"); err == nil {
		t.Error("ParseToken with garbage input: expected error, got nil")
	}
}

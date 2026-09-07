package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("VerifyPassword with correct password: %v", err)
	}
	if err := VerifyPassword(hash, "wrong password"); err == nil {
		t.Error("VerifyPassword with wrong password: expected error, got nil")
	}
}

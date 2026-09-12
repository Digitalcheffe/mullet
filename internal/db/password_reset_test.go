package db

import (
	"errors"
	"testing"
	"time"
)

func TestPasswordResetTokenRoundTrip(t *testing.T) {
	sqldb := newTestDB(t)
	result, err := sqldb.Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	id64, _ := result.LastInsertId()
	userID := int(id64)

	token, err := CreatePasswordResetToken(sqldb, userID)
	if err != nil {
		t.Fatalf("CreatePasswordResetToken: %v", err)
	}
	if token == "" {
		t.Fatal("CreatePasswordResetToken returned an empty token")
	}

	gotUserID, err := ConsumePasswordResetToken(sqldb, token)
	if err != nil {
		t.Fatalf("ConsumePasswordResetToken: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("ConsumePasswordResetToken userID = %d, want %d", gotUserID, userID)
	}

	// Single-use: redeeming the same token again must fail.
	if _, err := ConsumePasswordResetToken(sqldb, token); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("second ConsumePasswordResetToken = %v, want ErrResetTokenInvalid", err)
	}
}

func TestConsumePasswordResetTokenUnknownReturnsInvalid(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := ConsumePasswordResetToken(sqldb, "not-a-real-token"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("ConsumePasswordResetToken(unknown) = %v, want ErrResetTokenInvalid", err)
	}
}

func TestConsumePasswordResetTokenExpiredReturnsInvalid(t *testing.T) {
	sqldb := newTestDB(t)
	result, err := sqldb.Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	id64, _ := result.LastInsertId()
	userID := int(id64)

	token, err := CreatePasswordResetToken(sqldb, userID)
	if err != nil {
		t.Fatalf("CreatePasswordResetToken: %v", err)
	}
	// Back-date the token past its TTL directly, rather than waiting an
	// hour in a test.
	if _, err := sqldb.Exec(`UPDATE password_reset_tokens SET expires_at = ? WHERE token_hash = ?`, time.Now().Add(-time.Minute), hashResetToken(token)); err != nil {
		t.Fatalf("backdating token: %v", err)
	}

	if _, err := ConsumePasswordResetToken(sqldb, token); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("ConsumePasswordResetToken(expired) = %v, want ErrResetTokenInvalid", err)
	}
}

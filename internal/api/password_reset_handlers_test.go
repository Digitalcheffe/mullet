package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func TestForgotPasswordWithoutSMTPConfiguredReturns503(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(forgotPasswordRequest{Username: "admin"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/forgot-password", bytes.NewReader(body)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestForgotPasswordUnknownUsernameReturns204(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	configureFakeSMTP(t, router)

	body, _ := json.Marshal(forgotPasswordRequest{Username: "nobody"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/forgot-password", bytes.NewReader(body)))
	// Same response as a real send -- doesn't confirm or deny a specific
	// username exists.
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestForgotPasswordAccountWithoutEmailReturns422(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	configureFakeSMTP(t, router)

	// newTestUserDB seeds an "admin" user with no email set.
	body, _ := json.Marshal(forgotPasswordRequest{Username: "admin"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/forgot-password", bytes.NewReader(body)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestForgotPasswordSendsResetEmail(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)

	rec := httptest.NewRecorder()
	emailBody, _ := json.Marshal(updateAccountEmailRequest{Email: "admin@example.com"})
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", emailBody))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("setting account email: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	body, _ := json.Marshal(forgotPasswordRequest{Username: "admin"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/forgot-password", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("forgot-password: status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM password_reset_tokens`).Scan(&count); err != nil {
		t.Fatalf("counting reset tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("password_reset_tokens has %d rows, want 1", count)
	}
}

func TestResetPasswordRejectsShortPassword(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(resetPasswordRequest{Token: "whatever", NewPassword: "short"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/reset-password", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordRejectsUnknownToken(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(resetPasswordRequest{Token: "not-a-real-token", NewPassword: "newpassword123"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/reset-password", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordActuallyChangesPassword(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)

	user, err := db.GetUserByUsername(sqldb, "admin")
	if err != nil {
		t.Fatalf("looking up seeded admin: %v", err)
	}
	token, err := db.CreatePasswordResetToken(sqldb, user.ID)
	if err != nil {
		t.Fatalf("creating reset token: %v", err)
	}

	body, _ := json.Marshal(resetPasswordRequest{Token: token, NewPassword: "brandnewpassword"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/reset-password", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reset-password: status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	// The same token can't be redeemed twice.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/reset-password", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reusing the token: status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "brandnewpassword"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	if rec.Code != http.StatusOK {
		t.Errorf("login with new password: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// The old password must no longer work.
	oldLoginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(oldLoginBody)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password: status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

// configureFakeSMTP saves a working SMTP config pointed at a local fake
// server, so forgot-password's send step can succeed in a test.
func configureFakeSMTP(t *testing.T, router http.Handler) {
	t.Helper()
	host, port := startFakeSMTPServer(t)
	body, _ := json.Marshal(smtpConfigRequest{Host: host, Port: port, FromAddress: "mullet@example.com", TLSMode: "none"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("configuring fake smtp: status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

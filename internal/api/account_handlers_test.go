package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateAccountEmailRoundTrip(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(updateAccountEmailRequest{Email: "admin@example.com"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/me", nil))
	var me struct {
		Email *string `json:"email"`
	}
	json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Email == nil || *me.Email != "admin@example.com" {
		t.Errorf("GET /api/admin/me email = %v, want admin@example.com", me.Email)
	}
}

func TestUpdateAccountEmailRejectsInvalidAddress(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(updateAccountEmailRequest{Email: "not an email"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestUpdateAccountEmailAllowsClearing(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(updateAccountEmailRequest{Email: "admin@example.com"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("initial put status = %d, want 204", rec.Code)
	}

	body, _ = json.Marshal(updateAccountEmailRequest{Email: ""})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clearing put status = %d, want 204", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/me", nil))
	var me struct {
		Email *string `json:"email"`
	}
	json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Email != nil {
		t.Errorf("email = %v, want nil after clearing", me.Email)
	}
}

func TestUpdateAccountPasswordRoundTrip(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(updateAccountPasswordRequest{CurrentPassword: "s3cret", NewPassword: "newpassword1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/password", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	newLoginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "newpassword1"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(newLoginBody)))
	if rec.Code != http.StatusOK {
		t.Errorf("login with new password: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	oldLoginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(oldLoginBody)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password: status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestUpdateAccountPasswordRejectsWrongCurrentPassword(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(updateAccountPasswordRequest{CurrentPassword: "wrong-password", NewPassword: "newpassword1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/password", body))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestUpdateAccountPasswordRejectsShortNewPassword(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(updateAccountPasswordRequest{CurrentPassword: "s3cret", NewPassword: "short"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/password", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

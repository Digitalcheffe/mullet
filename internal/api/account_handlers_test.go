package api

import (
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

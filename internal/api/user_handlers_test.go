package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestListUsersReturnsSeededAdmin(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/users", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// Never leak the password hash, whatever the field name might be.
	if strings.Contains(strings.ToLower(rec.Body.String()), "hash") {
		t.Errorf("response body contains %q, want no password hash: %s", "hash", rec.Body.String())
	}

	var users []userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(users) != 1 || users[0].Username != "admin" {
		t.Errorf("users = %+v, want one seeded admin", users)
	}
}

func TestCreateUserRoundTrip(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(createUserRequest{Username: "bob", Password: "longenough1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/users", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding created user: %v", err)
	}
	if created.Username != "bob" || created.ID == 0 {
		t.Errorf("created = %+v, want username bob and a real id", created)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/users", nil))
	var users []userResponse
	json.Unmarshal(rec.Body.Bytes(), &users)
	if len(users) != 2 {
		t.Errorf("users after create = %+v, want 2 accounts", users)
	}

	// The new account can actually log in.
	loginBody, _ := json.Marshal(loginRequest{Username: "bob", Password: "longenough1"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	if rec.Code != http.StatusOK {
		t.Errorf("login as new user: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateUserRejectsShortPassword(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(createUserRequest{Username: "bob", Password: "short"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/users", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateUserRejectsDuplicateUsername(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(createUserRequest{Username: "admin", Password: "longenough1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/users", body))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteUserRefusesLastAdmin(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/users/1", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteUserRemovesSecondAccount(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(createUserRequest{Username: "bob", Password: "longenough1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/users", body))
	var created userResponse
	json.Unmarshal(rec.Body.Bytes(), &created)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/users/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/users", nil))
	var users []userResponse
	json.Unmarshal(rec.Body.Bytes(), &users)
	if len(users) != 1 || users[0].Username != "admin" {
		t.Errorf("users after delete = %+v, want just admin", users)
	}
}

func TestDeleteUserUnknownReturns404(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/users/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

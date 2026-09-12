package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func TestGetNotificationPreferencesDefaultsToAllOff(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/notifications", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var prefs db.NotificationPreferences
	json.Unmarshal(rec.Body.Bytes(), &prefs)
	if prefs != (db.NotificationPreferences{}) {
		t.Errorf("preferences = %+v, want all false before anything was saved", prefs)
	}
}

func TestNotificationPreferencesRoundTrip(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	want := db.NotificationPreferences{NewUser: true, ClientRegistered: true}
	body, _ := json.Marshal(want)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/notifications", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/notifications", nil))
	var got db.NotificationPreferences
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got != want {
		t.Errorf("preferences after save = %+v, want %+v", got, want)
	}
}

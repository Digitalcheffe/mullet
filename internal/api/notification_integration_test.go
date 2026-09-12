package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// These tests exercise the wiring from each triggering handler into
// internal/notify, not notify's own email content (see
// internal/notify/notify_test.go for that) -- the concern here is just
// that enabling a notification preference doesn't break or slow down
// the primary action it's attached to.

func TestCreateUserSucceedsWithNewUserNotificationEnabled(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{NewUser: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	body, _ := json.Marshal(createUserRequest{Username: "bob", Password: "longenough1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/users", body))
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestRegisterClientSucceedsWithClientRegisteredNotificationEnabled(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientRegistered: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	first := registerTestClient(t, router, "x7k-m2p")
	if first.Status != "pending" {
		t.Errorf("first registration = %+v, want pending", first)
	}

	// A repeat registration of the same client_id must not re-notify or
	// error just because it's not "new" the second time.
	second := registerTestClient(t, router, "x7k-m2p")
	if second.ID != first.ID {
		t.Errorf("re-registration created a new row: first.ID=%d second.ID=%d", first.ID, second.ID)
	}
}

func TestApproveClientSucceedsWithClientApprovedNotificationEnabled(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientApproved: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	displayID := createTestDisplay(t, router)
	c := registerTestClient(t, router, "x7k-m2p")

	approveBody, _ := json.Marshal(approveClientRequest{DisplayID: displayID})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", approveBody))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestForgotPasswordSucceedsWithPasswordResetRequestedNotificationEnabled(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{PasswordResetRequested: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	emailBody, _ := json.Marshal(updateAccountEmailRequest{Email: "admin@example.com"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/account/email", emailBody))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("setting account email: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	body, _ := json.Marshal(forgotPasswordRequest{Username: "admin"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/forgot-password", bytes.NewReader(body)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordSucceedsWithPasswordResetCompletedNotificationEnabled(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	configureFakeSMTP(t, router)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{PasswordResetCompleted: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

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
		t.Errorf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

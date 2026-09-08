package oauth

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	return sqldb
}

func TestEnsureFreshTokenReturnsUnexpiredTokenWithoutRefreshing(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := db.CreatePluginInstance(sqldb, "msgraph-calendar", "Work", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	if err := db.UpsertOAuthToken(sqldb, instanceID, "still-good", nil, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	// No token server configured -- if this tried to refresh, it would
	// fail to connect and this test would catch that as an error.
	got, err := EnsureFreshToken(t.Context(), sqldb, instanceID, Config{})
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if got != "still-good" {
		t.Errorf("got %q, want the stored token unchanged", got)
	}
}

func TestEnsureFreshTokenRefreshesExpiredToken(t *testing.T) {
	srv := fakeTokenServer(t)
	defer srv.Close()

	sqldb := newTestDB(t)
	instanceID, err := db.CreatePluginInstance(sqldb, "msgraph-calendar", "Work", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	refresh := "refresh-1"
	if err := db.UpsertOAuthToken(sqldb, instanceID, "expired", &refresh, time.Now().Add(-time.Minute), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	cfg := Config{TokenURL: srv.URL, ClientID: "client-abc", ClientSecret: "secret-xyz"}
	got, err := EnsureFreshToken(t.Context(), sqldb, instanceID, cfg)
	if err != nil {
		t.Fatalf("EnsureFreshToken: %v", err)
	}
	if got != "access-refreshed" {
		t.Errorf("got %q, want the refreshed access token", got)
	}

	// The refresh must be persisted, not just returned -- a later call
	// should see the new token without needing to refresh again.
	stored, err := db.GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if stored.AccessToken != "access-refreshed" || stored.RefreshToken == nil || *stored.RefreshToken != "refresh-2" {
		t.Errorf("stored = %+v, want the refreshed access+refresh token persisted", stored)
	}
}

func TestEnsureFreshTokenWithoutRefreshTokenFailsOnceExpired(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := db.CreatePluginInstance(sqldb, "msgraph-calendar", "Work", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	if err := db.UpsertOAuthToken(sqldb, instanceID, "expired", nil, time.Now().Add(-time.Minute), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	if _, err := EnsureFreshToken(t.Context(), sqldb, instanceID, Config{}); err == nil {
		t.Error("EnsureFreshToken with no refresh token = nil error, want failure")
	}
}

func TestEnsureFreshTokenUnauthorizedInstanceReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := db.CreatePluginInstance(sqldb, "msgraph-calendar", "Work", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	if _, err := EnsureFreshToken(t.Context(), sqldb, instanceID, Config{}); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("EnsureFreshToken(never authorized) = %v, want db.ErrNotFound", err)
	}
}

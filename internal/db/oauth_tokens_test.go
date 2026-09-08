package db

import (
	"errors"
	"testing"
	"time"
)

func TestOAuthTokenUpsertAndGet(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := CreatePluginInstance(sqldb, "msgraph-calendar", "Work Calendar", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	if _, err := GetOAuthToken(sqldb, instanceID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetOAuthToken before authorize = %v, want ErrNotFound", err)
	}

	refresh := "refresh-1"
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := UpsertOAuthToken(sqldb, instanceID, "access-1", &refresh, expires, "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	got, err := GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got.AccessToken != "access-1" || got.RefreshToken == nil || *got.RefreshToken != "refresh-1" || got.Scopes != "Calendars.Read" {
		t.Errorf("got = %+v, unexpected values", got)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, expires)
	}

	// A second upsert (e.g. a refresh cycle) replaces the row rather than
	// erroring on the UNIQUE plugin_instance_id constraint.
	refresh2 := "refresh-2"
	expires2 := expires.Add(time.Hour)
	if err := UpsertOAuthToken(sqldb, instanceID, "access-2", &refresh2, expires2, "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken (refresh): %v", err)
	}
	got, err = GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken after refresh: %v", err)
	}
	if got.AccessToken != "access-2" || *got.RefreshToken != "refresh-2" {
		t.Errorf("after refresh: got = %+v, want access-2/refresh-2", got)
	}
}

func TestOAuthTokenWithoutRefreshToken(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := CreatePluginInstance(sqldb, "msgraph-calendar", "Work Calendar", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	if err := UpsertOAuthToken(sqldb, instanceID, "access-1", nil, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}
	got, err := GetOAuthToken(sqldb, instanceID)
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got.RefreshToken != nil {
		t.Errorf("RefreshToken = %v, want nil", *got.RefreshToken)
	}
}

func TestDeleteOAuthToken(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := CreatePluginInstance(sqldb, "msgraph-calendar", "Work Calendar", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	if err := UpsertOAuthToken(sqldb, instanceID, "access-1", nil, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	if err := DeleteOAuthToken(sqldb, instanceID); err != nil {
		t.Fatalf("DeleteOAuthToken: %v", err)
	}
	if _, err := GetOAuthToken(sqldb, instanceID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetOAuthToken after delete = %v, want ErrNotFound", err)
	}

	// Deleting a token that never existed is a no-op, not an error.
	if err := DeleteOAuthToken(sqldb, instanceID); err != nil {
		t.Errorf("DeleteOAuthToken (already gone): %v", err)
	}
}

func TestDeletingPluginInstanceCascadesToOAuthToken(t *testing.T) {
	sqldb := newTestDB(t)
	instanceID, err := CreatePluginInstance(sqldb, "msgraph-calendar", "Work Calendar", 300, true, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	if err := UpsertOAuthToken(sqldb, instanceID, "access-1", nil, time.Now().Add(time.Hour), "Calendars.Read"); err != nil {
		t.Fatalf("UpsertOAuthToken: %v", err)
	}

	if err := DeletePluginInstance(sqldb, instanceID); err != nil {
		t.Fatalf("DeletePluginInstance: %v", err)
	}
	if _, err := GetOAuthToken(sqldb, instanceID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetOAuthToken after instance delete = %v, want ErrNotFound (cascade)", err)
	}
}

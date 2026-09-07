package db

import "testing"

func TestGetSetSetting(t *testing.T) {
	sqldb := newTestDB(t)

	if _, found, err := GetSetting(sqldb, "missing_key"); err != nil {
		t.Fatalf("GetSetting (missing): %v", err)
	} else if found {
		t.Error("GetSetting (missing) returned found=true")
	}

	if err := SetSetting(sqldb, "server_name", "Living Room"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	value, found, err := GetSetting(sqldb, "server_name")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !found || value != "Living Room" {
		t.Errorf("GetSetting = (%q, %v), want (\"Living Room\", true)", value, found)
	}

	if err := SetSetting(sqldb, "server_name", "Kitchen"); err != nil {
		t.Fatalf("SetSetting (update): %v", err)
	}
	value, _, err = GetSetting(sqldb, "server_name")
	if err != nil {
		t.Fatalf("GetSetting (after update): %v", err)
	}
	if value != "Kitchen" {
		t.Errorf("value after update = %q, want %q", value, "Kitchen")
	}
}

package auth

import (
	"path/filepath"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func TestLoadOrCreateJWTSecretPersists(t *testing.T) {
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqldb.Close()
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	first, err := LoadOrCreateJWTSecret(sqldb)
	if err != nil {
		t.Fatalf("LoadOrCreateJWTSecret (first): %v", err)
	}
	if len(first) != 32 {
		t.Errorf("secret length = %d, want 32", len(first))
	}

	second, err := LoadOrCreateJWTSecret(sqldb)
	if err != nil {
		t.Fatalf("LoadOrCreateJWTSecret (second): %v", err)
	}

	if string(first) != string(second) {
		t.Error("second call returned a different secret; expected it to persist")
	}
}

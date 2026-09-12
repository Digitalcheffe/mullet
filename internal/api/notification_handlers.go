package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// handleGetNotificationPreferences returns the saved toggles, or all
// false if nothing has been saved yet.
func handleGetNotificationPreferences(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, err := db.GetNotificationPreferences(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prefs)
	}
}

// handlePutNotificationPreferences saves the toggles wholesale --
// there's no partial-update path, so the frontend always echoes back
// every field.
func handlePutNotificationPreferences(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var prefs db.NotificationPreferences
		if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := db.SaveNotificationPreferences(sqldb, prefs); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Package api wires up the HTTP router serving the data API, admin API,
// client API, and display endpoints.
package api

import (
	"database/sql"
	"net/http"
)

// NewRouter builds the top-level HTTP handler for the server. jwtSecret
// signs and verifies admin session tokens; corsOrigins configures which
// cross-origin callers may access the API (empty disables CORS headers).
func NewRouter(sqldb *sql.DB, jwtSecret []byte, corsOrigins []string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	// /api/admin/* -- login is public; everything else requires a valid
	// JWT. Real admin endpoints (settings, users, plugins, displays, ...)
	// are added in later issues; handleWhoAmI exercises the auth guard
	// end-to-end in the meantime.
	mux.HandleFunc("POST /api/admin/login", handleLogin(sqldb, jwtSecret))

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/admin/me", handleWhoAmI)
	mux.Handle("/api/admin/", requireAuth(jwtSecret)(adminMux))

	// /api/data/* -- served to the display frontend from typed shape
	// tables. Handlers land in issue #14.
	dataMux := http.NewServeMux()
	mux.Handle("/api/data/", dataMux)

	// /api/clients/* -- dedicated client app registration/polling, no
	// auth (clients are admin-approved). Handlers land in issue #29.
	clientsMux := http.NewServeMux()
	mux.Handle("/api/clients/", clientsMux)

	// /display/* -- renders the full-screen display view. Handler lands
	// in issue #23.
	displayMux := http.NewServeMux()
	mux.Handle("/display/", displayMux)

	return withLogging(withCORS(corsOrigins)(mux))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

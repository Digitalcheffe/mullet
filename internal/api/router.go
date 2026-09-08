// Package api wires up the HTTP router serving the data API, admin API,
// client API, and display endpoints.
package api

import (
	"database/sql"
	"net/http"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
)

// ServerInfo carries server bootstrap facts the admin API reports but
// can't change at runtime (the listening port, DB path) alongside when
// the process started, for uptime reporting.
type ServerInfo struct {
	Port      string
	DBPath    string
	StartedAt time.Time
}

// NewRouter builds the top-level HTTP handler for the server. jwtSecret
// signs and verifies admin session tokens; corsOrigins configures which
// cross-origin callers may access the API (empty disables CORS headers);
// staticDir is the built frontend (web/dist) served for /admin, /display,
// and everything else not matched below. authDisabled skips setup/login
// and lets every /api/admin/* request through unauthenticated -- local
// dev only, never set this in a real deployment. registry and sched back
// the plugin management endpoints: sched.Reload() is called after any
// instance create/update/delete so changes take effect without a
// restart.
func NewRouter(sqldb *sql.DB, jwtSecret []byte, corsOrigins []string, info ServerInfo, staticDir string, authDisabled bool, registry *plugindata.Registry, sched *scheduler.Scheduler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	// /api/admin/* -- login and first-run setup are public; everything
	// else requires a valid JWT (unless authDisabled). Real admin
	// endpoints (users, displays, ...) are added in later issues;
	// handleWhoAmI exercises the auth guard end-to-end in the meantime.
	mux.HandleFunc("POST /api/admin/login", handleLogin(sqldb, jwtSecret))
	mux.HandleFunc("GET /api/admin/setup", handleSetupStatus(sqldb, authDisabled))
	mux.HandleFunc("POST /api/admin/setup", handleSetup(sqldb, jwtSecret))

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/admin/me", handleWhoAmI)
	adminMux.HandleFunc("GET /api/admin/dashboard", handleDashboard(sqldb, info))
	adminMux.HandleFunc("GET /api/admin/settings", handleGetSettings(sqldb, info))
	adminMux.HandleFunc("PUT /api/admin/settings", handlePutSettings(sqldb))
	adminMux.HandleFunc("GET /api/admin/plugins", handleListPlugins(registry))
	adminMux.HandleFunc("GET /api/admin/plugins/instances", handleListPluginInstances(sqldb))
	adminMux.HandleFunc("POST /api/admin/plugins/instances", handleCreatePluginInstance(sqldb, registry, sched))
	adminMux.HandleFunc("PUT /api/admin/plugins/instances/{id}", handleUpdatePluginInstance(sqldb, registry, sched))
	adminMux.HandleFunc("DELETE /api/admin/plugins/instances/{id}", handleDeletePluginInstance(sqldb, sched))
	adminMux.HandleFunc("POST /api/admin/plugins/instances/{id}/test", handleTestPluginInstance(sched))

	adminMux.HandleFunc("GET /api/admin/themes", handleListThemes(sqldb))
	adminMux.HandleFunc("POST /api/admin/themes", handleCreateTheme(sqldb))
	adminMux.HandleFunc("PUT /api/admin/themes/{id}", handleUpdateTheme(sqldb))
	adminMux.HandleFunc("DELETE /api/admin/themes/{id}", handleDeleteTheme(sqldb))

	adminMux.HandleFunc("GET /api/admin/displays", handleListDisplays(sqldb))
	adminMux.HandleFunc("POST /api/admin/displays", handleCreateDisplay(sqldb))
	adminMux.HandleFunc("PUT /api/admin/displays/{id}", handleUpdateDisplay(sqldb))
	adminMux.HandleFunc("DELETE /api/admin/displays/{id}", handleDeleteDisplay(sqldb))

	adminMux.HandleFunc("GET /api/admin/displays/{id}/screens", handleListScreens(sqldb))
	adminMux.HandleFunc("POST /api/admin/displays/{id}/screens", handleCreateScreen(sqldb))
	adminMux.HandleFunc("PUT /api/admin/screens/{id}", handleUpdateScreen(sqldb))
	adminMux.HandleFunc("DELETE /api/admin/screens/{id}", handleDeleteScreen(sqldb))

	adminMux.HandleFunc("GET /api/admin/screens/{id}/cards", handleListCards(sqldb))
	adminMux.HandleFunc("POST /api/admin/screens/{id}/cards", handleCreateCard(sqldb))
	adminMux.HandleFunc("PUT /api/admin/cards/{id}", handleUpdateCard(sqldb))
	adminMux.HandleFunc("DELETE /api/admin/cards/{id}", handleDeleteCard(sqldb))

	mux.Handle("/api/admin/", requireAuth(jwtSecret, authDisabled)(adminMux))

	// /api/data/* -- served to the display frontend directly from typed
	// shape tables, no auth (LAN-facing, like the display itself).
	dataMux := http.NewServeMux()
	dataMux.HandleFunc("GET /api/data/{shape}", handleGetShapeData(sqldb))
	mux.Handle("/api/data/", dataMux)

	// /api/clients/* -- dedicated client app registration/polling, no
	// auth (clients are admin-approved). Handlers land in issue #29.
	clientsMux := http.NewServeMux()
	mux.Handle("/api/clients/", clientsMux)

	// Everything else -- /admin, /display/{slug}, and their static assets
	// -- is the built frontend. Both are client-side routed (react-router),
	// so any path without a matching static file falls back to
	// index.html. /display/{slug} itself just renders the SPA shell here;
	// the display's actual layout comes from a data API added in #18/#23.
	mux.Handle("/", newSPAHandler(staticDir))

	return withLogging(withCORS(corsOrigins)(mux))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

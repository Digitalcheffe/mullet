package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/notify"
)

// clientPollIntervalSeconds is the poll interval every client is told
// to use -- fixed and server-wide rather than per-client configurable,
// matching docs/architecture_1.md's documented default. Nothing in the
// clients table stores it; a future issue can make it configurable if
// a real need shows up.
const clientPollIntervalSeconds = 15

// clientOnlineThreshold is how recently last_seen_at must have been
// touched for a client to be reported online. A client that's actually
// offline has no way to tell the server so (see clients.go's own doc
// comment on Client.Status), so this is the only signal available --
// generously wide (4x the poll interval) to avoid flapping between
// online/offline on an ordinary slow poll.
const clientOnlineThreshold = 4 * clientPollIntervalSeconds * time.Second

var validClientStatuses = map[string]bool{"pending": true, "approved": true, "rejected": true}
var validOfflineModes = map[string]bool{"offline_screen": true, "screen_off": true, "last_screenshot": true}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isClientOnline(c db.Client) bool {
	return c.Status == "approved" && c.LastSeenAt != nil && time.Since(*c.LastSeenAt) < clientOnlineThreshold
}

// clientIP reports the caller's address for a client-facing request --
// shown to the admin alongside a pending client's pairing code so two
// pending clients with the same generic name (two browser tabs both
// registered as "Kitchen") can still be told apart before approving
// one. Prefers X-Forwarded-For's first hop (the original client,
// per the standard reverse-proxy convention) when set -- Mullet itself
// never sets this header, so it's only present when a reverse proxy in
// front of it adds one -- otherwise falls back to the direct TCP peer.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first, _, ok := strings.Cut(fwd, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- Client-facing (public, no auth -- clients are admin-approved,
// not authenticated) ----

type clientRegisterRequest struct {
	ClientID   string `json:"client_id"`
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type clientResponse struct {
	ID          int     `json:"id"`
	ClientID    string  `json:"client_id"`
	Name        string  `json:"name"`
	DisplayID   *int    `json:"display_id,omitempty"`
	Status      string  `json:"status"`
	Online      bool    `json:"online"`
	LastSeenAt  *string `json:"last_seen_at,omitempty"`
	OfflineMode string  `json:"offline_mode"`
	Platform    *string `json:"platform,omitempty"`
	AppVersion  *string `json:"app_version,omitempty"`
	IPAddress   *string `json:"ip_address,omitempty"`
}

func toClientResponse(c db.Client) clientResponse {
	resp := clientResponse{
		ID: c.ID, ClientID: c.ClientID, Name: c.Name, DisplayID: c.DisplayID,
		Status: c.Status, Online: isClientOnline(c), OfflineMode: c.OfflineMode,
		Platform: c.Platform, AppVersion: c.AppVersion, IPAddress: c.IPAddress,
	}
	if c.LastSeenAt != nil {
		s := c.LastSeenAt.UTC().Format(time.RFC3339)
		resp.LastSeenAt = &s
	}
	return resp
}

// handleRegisterClient serves POST /api/clients/register: the client
// generates its own client_id locally (a short pairing code shown on
// screen, per docs/architecture_1.md's registration flow) and sends it
// here along with a display name and platform info. Always starts (or
// stays) "pending" -- approval and display assignment happen later, in
// the admin UI.
func handleRegisterClient(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clientRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ClientID == "" || req.Name == "" {
			http.Error(w, "client_id and name are required", http.StatusBadRequest)
			return
		}

		c, isNew, err := db.RegisterClient(sqldb, req.ClientID, req.Name, nullableString(req.Platform), nullableString(req.AppVersion), nullableString(clientIP(r)))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if isNew {
			if err := notify.ClientRegistered(sqldb, c.Name); err != nil {
				log.Printf("client %d registered but notification failed: %v", c.ID, err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toClientResponse(c))
	}
}

type clientConfigResponse struct {
	ClientID            string  `json:"client_id"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	DisplaySlug         *string `json:"display_slug,omitempty"`
	PollIntervalSeconds int     `json:"poll_interval_seconds"`
	OfflineMode         string  `json:"offline_mode"`
}

// handleClientConfig serves GET /api/clients/{client_id}/config: the
// client's polling heartbeat. Every poll stamps last_seen_at (this is
// the only signal the server has for online/offline, so it has to
// happen on every successful poll, not just registration) and reports
// the client's current status and display assignment -- a still-
// "pending" client polls this the same way an approved one does, just
// gets back no display_slug yet.
func handleClientConfig(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("client_id")
		c, err := db.GetClientByClientID(sqldb, clientID)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not registered", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := db.TouchClientLastSeen(sqldb, clientID, clientIP(r)); err != nil && !errors.Is(err, db.ErrNotFound) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := clientConfigResponse{
			ClientID: c.ClientID, Name: c.Name, Status: c.Status,
			PollIntervalSeconds: clientPollIntervalSeconds, OfflineMode: c.OfflineMode,
		}
		if c.Status == "approved" && c.DisplayID != nil {
			if d, err := db.GetDisplay(sqldb, *c.DisplayID); err == nil {
				resp.DisplaySlug = &d.Slug
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type clientOfflineResponse struct {
	HTML      string `json:"html"`
	UpdatedAt string `json:"updated_at"`
}

// handleClientOffline serves GET /api/clients/{client_id}/offline: the
// self-contained HTML page the client caches locally and shows when it
// can't reach the server (see docs/architecture_1.md's Offline Mode).
// Falls back to a generated default (clock + display name) unless the
// assigned display has a custom one configured.
func handleClientOffline(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("client_id")
		c, err := db.GetClientByClientID(sqldb, clientID)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not registered", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		label := c.Name
		var custom *string
		if c.DisplayID != nil {
			if d, err := db.GetDisplay(sqldb, *c.DisplayID); err == nil {
				label = d.Name
			}
			if h, err := db.GetDisplayOfflineScreenHTML(sqldb, *c.DisplayID); err == nil {
				custom = h
			}
		}

		html := defaultOfflineScreenHTML(label)
		if custom != nil {
			html = *custom
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clientOfflineResponse{HTML: html, UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	}
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// defaultOfflineScreenHTML is the auto-generated fallback offline page
// -- self-contained (inline styles, no external requests), a live
// clock plus the display's own name, styled as a generic dark kiosk
// screen rather than matching any particular display's theme (an admin
// who wants exact theme matching sets a custom page instead, via
// PUT /api/admin/displays/{id}/offline-screen).
func defaultOfflineScreenHTML(displayName string) string {
	return `<!doctype html>
<html><head><meta charset="utf-8"><style>
html,body{margin:0;height:100%;background:#0b0f14;color:#e6e9ee;font-family:system-ui,sans-serif;
  display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px}
.offline-time{font-size:4em;font-weight:700}
.offline-date{opacity:0.7}
.offline-name{font-size:1.4em;font-weight:600;margin-top:16px}
.offline-badge{position:fixed;top:16px;right:20px;font-size:0.9em;opacity:0.6}
.offline-note{opacity:0.5;font-size:0.9em;margin-top:24px}
</style></head>
<body>
<div class="offline-badge">&#9679; offline</div>
<div class="offline-time" id="t"></div>
<div class="offline-date" id="d"></div>
<div class="offline-name">` + htmlEscaper.Replace(displayName) + `</div>
<div class="offline-note">Waiting for server connection</div>
<script>
function tick(){
  var n=new Date();
  document.getElementById('t').textContent=n.toLocaleTimeString([], {hour:'numeric',minute:'2-digit'});
  document.getElementById('d').textContent=n.toLocaleDateString([], {weekday:'long',month:'long',day:'numeric'});
}
tick(); setInterval(tick, 1000);
</script>
</body></html>`
}

// ---- Admin client management (requires auth) ----

func handleListClients(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.ListClients(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]clientResponse, len(clients))
		for i, c := range clients {
			resp[i] = toClientResponse(c)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type approveClientRequest struct {
	DisplayID int `json:"display_id"`
}

func handleApproveClient(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req approveClientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.DisplayID == 0 {
			http.Error(w, "display_id is required", http.StatusBadRequest)
			return
		}

		switch err := db.ApproveClient(sqldb, id, req.DisplayID); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
			return
		case errors.Is(err, db.ErrInUse):
			http.Error(w, "display not found", http.StatusBadRequest)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if c, err := db.GetClient(sqldb, id); err != nil {
			log.Printf("client %d approved but looking it up for notification failed: %v", id, err)
		} else if err := notify.ClientApproved(sqldb, c.Name); err != nil {
			log.Printf("client %d approved but notification failed: %v", id, err)
		}

		writeClient(w, sqldb, id, http.StatusOK)
	}
}

type clientUpdateRequest struct {
	Name        string `json:"name"`
	DisplayID   *int   `json:"display_id"`
	Status      string `json:"status"`
	OfflineMode string `json:"offline_mode"`
}

func handleUpdateClient(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req clientUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if !validClientStatuses[req.Status] {
			http.Error(w, `status must be "pending", "approved", or "rejected"`, http.StatusBadRequest)
			return
		}
		if !validOfflineModes[req.OfflineMode] {
			http.Error(w, `offline_mode must be "offline_screen", "screen_off", or "last_screenshot"`, http.StatusBadRequest)
			return
		}

		switch err := db.UpdateClient(sqldb, id, req.Name, req.DisplayID, req.Status, req.OfflineMode); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
			return
		case errors.Is(err, db.ErrInUse):
			http.Error(w, "display not found", http.StatusBadRequest)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeClient(w, sqldb, id, http.StatusOK)
	}
}

func handleDeleteClient(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		switch err := db.DeleteClient(sqldb, id); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func writeClient(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	c, err := db.GetClient(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toClientResponse(c))
}

// ---- Admin per-display offline screen config (requires auth) ----

type offlineScreenResponse struct {
	HTML *string `json:"html"`
}

func handleGetDisplayOfflineScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		html, err := db.GetDisplayOfflineScreenHTML(sqldb, id)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(offlineScreenResponse{HTML: html})
	}
}

type offlineScreenRequest struct {
	HTML *string `json:"html"`
}

// handleSetDisplayOfflineScreen serves PUT
// /api/admin/displays/{id}/offline-screen. A null (or absent) html
// clears back to the generated default -- there's no separate DELETE
// endpoint for this since a null PUT already covers "reset it".
func handleSetDisplayOfflineScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req offlineScreenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.HTML != nil && strings.TrimSpace(*req.HTML) == "" {
			req.HTML = nil
		}

		if err := db.SetDisplayOfflineScreenHTML(sqldb, id, req.HTML); errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(offlineScreenResponse{HTML: req.HTML})
	}
}

# Mullet — GitHub Issue Plan

> **Repo:** `Digitalcheffe/mullet`
>
> **Total Issues:** 32
>
> **Label System:** Two dimensions — **type** (what kind of work) + **phase** (when it happens)

---

## Labels

### Type Labels

| Label | Color | Description |
|-------|-------|-------------|
| `type: infra` | `#0e8a16` | Project scaffolding, build system, CI/CD, Docker |
| `type: backend` | `#1d76db` | Go server code, database, APIs |
| `type: frontend` | `#d93f0b` | React/Vite, admin UI, display renderer |
| `type: plugin` | `#5319e7` | Data plugins or UI plugins |
| `type: api` | `#006b75` | REST API endpoints |
| `type: docs` | `#fbca04` | Documentation, guides, READMEs |

### Phase Labels

| Label | Color | Description |
|-------|-------|-------------|
| `phase: 1 — skeleton` | `#c2e0c6` | Skeleton + Admin Foundation |
| `phase: 2 — plugins` | `#bfd4f2` | Data Plugin Management |
| `phase: 3 — designer` | `#d4c5f9` | Display Designer |
| `phase: 4 — msgraph` | `#f9d0c4` | Microsoft Graph Integration |
| `phase: 5 — home` | `#fef2c0` | Home Integration |
| `phase: 6 — clients` | `#e6e6e6` | Client Support (Server-Side) |
| `phase: 7 — polish` | `#ededed` | Polish & Documentation |

---

## Phase 1: Skeleton + Admin Foundation

### Issue #1 — Go project scaffolding

**Labels:** `type: infra`, `phase: 1 — skeleton`

Set up the Go project structure with `cmd/server/main.go` entry point and `internal/` package layout. Initialize `go.mod`, wire up basic HTTP server, and add a health endpoint.

**Acceptance Criteria:**
- `cmd/server/main.go` starts an HTTP server on configurable port
- `internal/` package structure matches architecture doc (config, shapes, plugins, db, scheduler, auth, api)
- `go.mod` initialized with module path
- `GET /healthz` returns 200
- Makefile with `build`, `run`, `test` targets

---

### Issue #2 — Vite + React frontend scaffolding

**Labels:** `type: frontend`, `phase: 1 — skeleton`

Bootstrap the React frontend with Vite. Set up routing so `/admin/*` serves the admin UI and `/display/*` serves the display renderer. Both share a common `shared/` module.

**Acceptance Criteria:**
- `web/` directory with Vite + React + TypeScript
- Route split: admin app vs display app
- Shared types directory (`shared/types/`, `shared/hooks/`, `shared/themes/`)
- Hot reload works in dev mode
- Production build outputs static assets Go server can embed/serve

---

### Issue #3 — SQLite setup + migration system

**Labels:** `type: backend`, `phase: 1 — skeleton`

Implement SQLite database layer with a numbered migration system. Create the initial system tables (users, system settings).

**Acceptance Criteria:**
- SQLite connection pool with WAL mode
- Migration runner that applies numbered `.sql` files in order
- Tracks applied migrations in a `schema_migrations` table
- `001_initial.sql` creates `users` and `system_settings` tables
- DB path configurable via `DB_PATH` env var
- Auto-creates the database file if it doesn't exist

---

### Issue #4 — Typed data shape structs + SQLite tables

**Labels:** `type: backend`, `phase: 1 — skeleton`

Define the typed Go structs for each data shape and create their corresponding SQLite tables. Shapes are the contract between data plugins and UI plugins -- the framework owns them, not the plugins.

**Acceptance Criteria:**
- Typed Go structs for all v1 shapes: `CalendarEvent`, `Task`, `WeatherCurrent`, `WeatherForecast`, `HomeDevice`, `Package`, `InfraService`, `MediaStatus`
- Corresponding SQLite tables with typed columns: `shape_events`, `shape_tasks`, `shape_weather_current`, `shape_weather_forecast`, `shape_home_devices`, `shape_packages`, `shape_infrastructure`, `shape_media_status`
- Foreign keys to `data_plugin_instances` with `ON DELETE CASCADE`
- Metadata tables where needed (`calendars`, `task_lists`) with their own foreign keys
- Indexes on common query columns (calendar_id, task_list_id, start, date)
- Shapes are compile-time Go structs (not loaded from DB, not `map[string]any`)

---

### Issue #5 — Plugin interfaces + registry

**Labels:** `type: backend`, `phase: 1 — skeleton`

Define the `DataPlugin` and `DataPluginManifest` interfaces. Build the plugin registry for compiled-in plugins to register themselves at init time.

**Acceptance Criteria:**
- `DataPlugin` interface: `ID()`, `Name()`, `Manifest()`, `DataShape()`, `RefreshInterval()`, `Configure()`, `Fetch()`
- `DataPluginManifest` struct with `SetupFields` (text, select, multi-select, number, toggle, password)
- `OAuthConfig` struct for OAuth2-capable plugins
- Plugin registry: `Register()`, `List()`, `Get(id)`
- Registry used by scheduler and admin API

---

### Issue #6 — Scheduler

**Labels:** `type: backend`, `phase: 1 — skeleton`

Build the scheduler that runs data plugins on their configured intervals, writes results to the typed shape tables, and handles errors gracefully.

**Acceptance Criteria:**
- Starts/stops with the server
- Runs each enabled plugin instance on its `RefreshInterval`
- Calls `Fetch()`, receives typed Go structs from the plugin
- Writes results to the correct typed shape table (`shape_events`, `shape_tasks`, etc.) with `fetched_at` timestamp
- Logs errors per-plugin without crashing the scheduler
- Tracks last fetch time and error state per plugin instance
- Cleans stale data on successful refresh (replace, not append)

---

### Issue #7 — REST API skeleton + auth middleware

**Labels:** `type: api`, `phase: 1 — skeleton`

Set up the HTTP router with route groups for data API, admin API, client API, and display endpoints. Implement JWT auth middleware for admin routes.

**Acceptance Criteria:**
- Router with grouped routes: `/api/data/*`, `/api/admin/*`, `/api/clients/*`, `/display/*`
- CORS middleware (configurable origins)
- JWT-based auth middleware protecting admin routes
- Request logging middleware
- `POST /api/admin/login` returns JWT
- Admin routes return 401 without valid token

---

### Issue #8 — Admin UI: first-run setup wizard + login

**Labels:** `type: frontend`, `phase: 1 — skeleton`

Build the first-run experience: if no admin user exists, show a setup wizard to create one. After setup (or if an admin exists), show the login page.

**Acceptance Criteria:**
- First-run detection: API returns whether setup is needed
- Setup wizard: create admin username + password
- Login page with username/password form
- JWT stored in memory (not localStorage), refreshed on session
- Auth context provider wrapping admin routes
- Redirect to login on 401

---

### Issue #9 — Admin UI: system settings + admin dashboard

**Labels:** `type: frontend`, `phase: 1 — skeleton`

Build the admin shell (sidebar nav, top bar) and the two foundational pages: a system overview dashboard and a settings page.

**Acceptance Criteria:**
- Admin shell with sidebar navigation (links to all admin sections)
- Dashboard page: server uptime, plugin count, display count, active clients
- Settings page: server name, port display, DB path info
- Responsive layout (works on tablet for quick admin from couch)

---

### Issue #10 — Docker build (multi-stage)

**Labels:** `type: infra`, `phase: 1 — skeleton`

Create the multi-stage Dockerfile and `docker-compose.yml`. Go build → Vite build → Alpine runtime image with embedded assets.

**Acceptance Criteria:**
- Multi-stage Dockerfile: Go builder → Node builder → Alpine runtime
- Final image under 60MB
- `docker-compose.yml` with volume mount for SQLite persistence
- Env var passthrough for `PORT`, `DB_PATH`, and plugin secrets
- Builds for both `amd64` and `arm64` (Pi support)
- Single `docker compose up` starts everything

---

## Phase 2: Data Plugin Management

### Issue #11 — Clock plugin (prove the pattern)

**Labels:** `type: plugin`, `phase: 2 — plugins`

Build the simplest possible plugin: a system clock that returns the current time. No external API, no auth, no data shape. This proves the full plugin lifecycle end-to-end.

**Acceptance Criteria:**
- `clock` plugin implementing `DataPlugin` interface
- Manifest with zero setup fields (nothing to configure)
- Returns current time on `Fetch()`
- Registers itself in the plugin registry at init
- Scheduler runs it on interval
- Visible in admin plugin list

---

### Issue #12 — OpenWeatherMap data plugin + manifest

**Labels:** `type: plugin`, `phase: 2 — plugins`

Build the first real data plugin. Fetches current weather and forecast from OpenWeatherMap API, returns data conforming to `weather_current` and `weather_forecast` shapes.

**Acceptance Criteria:**
- Manifest with setup fields: API key (password type), location (text), units (select: metric/imperial)
- `auth_type: "api_key"`
- Fetches current weather → `weather_current` shape
- Fetches 5-day forecast → `weather_forecast` shape
- Proper error handling for invalid API key, rate limits, network failures
- Refresh interval: 15 minutes default

---

### Issue #13 — Admin UI: plugin management pages

**Labels:** `type: frontend`, `phase: 2 — plugins`

Build the admin pages for browsing, adding, configuring, and monitoring data plugin instances.

**Acceptance Criteria:**
- **Plugin list page:** shows all available plugins (compiled-in), enabled instances, and status
- **Manifest-driven setup wizard:** renders `SetupFields` from any plugin's manifest dynamically — text inputs, selects, multi-selects, toggles, password fields, help text
- **Plugin status dashboard:** per-instance last fetch time, error count, row count, enable/disable toggle
- Creating a plugin instance saves config to `plugin_instances` table
- Editing re-renders the manifest form with current values

---

### Issue #14 — Data API endpoints

**Labels:** `type: api`, `phase: 2 — plugins`

Implement the data API that serves data from typed shape tables to the display frontend.

**Acceptance Criteria:**
- `GET /api/data/{shape}` — returns all data for a shape from its typed table (e.g., `/api/data/events` queries `shape_events`)
- `GET /api/data/{shape}?plugin={instance_id}` — filter by plugin instance
- Response format: `{ data: [...], last_updated: "...", source: "..." }`
- Queries typed SQLite tables directly, never calls plugins
- Returns 404 for unknown shapes, empty array for shapes with no data
- Supports shape-specific query params where useful (e.g., `?from=&to=` for events)

---

### Issue #15 — Admin plugin management API

**Labels:** `type: api`, `phase: 2 — plugins`

Build the admin API endpoints for CRUD operations on plugin instances.

**Acceptance Criteria:**
- `GET /api/admin/plugins` — list available plugins + their manifests
- `GET /api/admin/plugins/instances` — list configured instances with status
- `POST /api/admin/plugins/instances` — create new instance (validate against manifest)
- `PUT /api/admin/plugins/instances/{id}` — update instance config
- `DELETE /api/admin/plugins/instances/{id}` — remove instance
- `POST /api/admin/plugins/instances/{id}/test` — run a one-off fetch to verify config

---

### Issue #16 — ICS feed data plugin

**Labels:** `type: plugin`, `phase: 2 — plugins`

Build the ICS/iCal feed data plugin. This is the universal calendar connector -- works with any service that exposes an ICS URL (Outlook personal, Google Calendar, Apple Calendar, self-hosted). No OAuth required, just a URL.

**Acceptance Criteria:**
- Manifest setup fields: ICS URL (text, supports multiple), display name per feed (text), color per feed (color picker)
- Fetches and parses ICS feeds using a Go iCal library
- Handles RRULE recurring events, EXDATE exceptions, VTIMEZONE, all-day events (DTSTART;VALUE=DATE)
- Writes to `shape_events` table via `CalendarEvent` struct
- Creates entries in `calendars` metadata table per feed
- Refresh interval: 15 minutes default
- Error handling: invalid URL, malformed ICS, network timeout

---

### Issue #17 — Architecture doc in repo

**Labels:** `type: docs`, `phase: 2 — plugins`

Add the architecture.md design document to the repository root as the canonical reference for contributors and future Claude sessions.

**Acceptance Criteria:**
- `architecture.md` in repo root
- Covers: plugin system, data shapes, display hierarchy, SQLite schema, REST API routes, theme cascade, client connection model
- Kept up to date as implementation diverges from initial design

---

## Phase 3: Display Designer

### Issue #18 — Display, screen, and card CRUD

**Labels:** `type: backend`, `phase: 3 — designer`

Implement the database layer and admin API for the display hierarchy: displays → screens → cards.

**Acceptance Criteria:**
- SQLite tables: `displays`, `screens`, `cards`, `themes` with proper foreign keys
- Full CRUD operations via admin API for each level
- Display has: name, slug (unique), theme_id, rotation interval
- Screen has: display_id, position, columns, row_height, gap
- Card has: screen_id, ui_plugin_id, data_plugin_instance_id, grid position (x, y, w, h), config JSON, theme overrides
- Cascade deletes: removing a display removes its screens and cards

---

### Issue #19 — Admin UI: display list + screen manager

**Labels:** `type: frontend`, `phase: 3 — designer`

Build the admin pages for managing displays and their screens.

**Acceptance Criteria:**
- **Display list:** create/edit/delete displays, set slug, assign theme
- **Screen manager:** within a display, add/reorder/delete screens
- Per-screen settings: column count, row height, gap size
- Display-level settings: rotation interval between screens, top bar / bottom bar toggles

---

### Issue #20 — Admin UI: drag-and-drop grid editor

**Labels:** `type: frontend`, `phase: 3 — designer`

Build the visual display designer — the drag-and-drop grid editor where users place, resize, and configure cards on a screen.

**Acceptance Criteria:**
- Grid surface reflecting the screen's column/row configuration
- Drag UI plugins from a sidebar palette onto the grid
- Resize cards by dragging edges/corners
- Move cards by dragging
- Collision detection (no overlapping cards)
- Card settings panel: pick data source (plugin instance), set UI plugin config from `configSchema`, apply theme overrides
- Live preview of the screen layout
- Save layout to SQLite

---

### Issue #21 — Theme editor

**Labels:** `type: frontend`, `phase: 3 — designer`

Build the visual theme token editor with live preview. Themes cascade: display sets base, cards inherit, cards can override.

**Acceptance Criteria:**
- Theme token editor: background, card background, text colors, accent, font family, border radius, opacity
- Live preview panel showing a sample card grid with the theme applied
- Save themes to `themes` table
- Assign themes to displays
- Per-card theme override in the card settings panel
- Bundled default theme (dark glass)

---

### Issue #22 — Weather UI plugins

**Labels:** `type: plugin`, `phase: 3 — designer`

Build the first UI plugins to prove the display renderer works end-to-end.

**Acceptance Criteria:**
- `weather-current` UI plugin: current temp, condition icon, high/low, humidity, wind
- `weather-forecast` UI plugin: 5-day forecast with day labels, icons, highs/lows
- Both consume their respective data shapes
- Both respect theme tokens (card bg, text colors, accent)
- Configurable via `configSchema` (e.g., show/hide humidity)
- Responsive within their grid cell (looks good at various card sizes)

---

### Issue #23 — Display renderer + grid engine

**Labels:** `type: frontend`, `phase: 3 — designer`

Build the display-side React app: the grid engine that reads a display layout from the API and renders it full-screen.

**Acceptance Criteria:**
- `GET /display/{slug}` serves the display React app
- Grid engine reads layout from API, renders cards in CSS Grid
- Screen rotation with configurable interval and transition animation
- Top bar (clock, weather summary) and bottom bar (alerts, status) if enabled
- Full-screen, no scrollbars, no browser chrome visible
- Auto-refreshes data on interval
- Built-in "reconnecting..." overlay when API is unreachable (for raw browser scenario)

---

## Phase 4: Microsoft Graph

### Issue #24 — OAuth2 flow handler

**Labels:** `type: backend`, `phase: 4 — msgraph`

Build a generic, reusable OAuth2 flow handler that any data plugin can use. Handles authorization code grant, token storage, and auto-refresh.

**Acceptance Criteria:**
- Generic OAuth2 handler driven by plugin manifest's `OAuthConfig`
- Authorization URL generation with correct scopes
- Callback handler that exchanges code for tokens
- Token storage in `oauth_tokens` table (per plugin instance)
- Automatic token refresh before expiry
- Admin UI: "Authorize" button in plugin setup wizard triggers the OAuth flow
- Works for both Microsoft consumer (`consumers` tenant) and enterprise (custom tenant GUID)
- Uses `msgraph-sdk-go` and MSAL for Microsoft-specific flows

---

### Issue #25 — Microsoft Graph data plugins

**Labels:** `type: plugin`, `phase: 4 — msgraph`

Build the Microsoft Graph calendar and to-do data plugins using the official `msgraph-sdk-go`.

**Acceptance Criteria:**
- `msgraph-calendar` plugin: fetches events from Outlook/Microsoft 365 calendar → `events` shape
- Manifest setup fields: tenant ID (text, with help text for consumer vs enterprise), calendar selection (multi-select, populated after OAuth)
- Handles recurring events, all-day events, multi-calendar
- `msgraph-todo` plugin: fetches tasks from Microsoft To-Do → `tasks` shape
- Manifest setup fields: tenant ID, task list selection
- Both plugins share the same OAuth2 credential (one authorization covers both)

---

### Issue #26 — Calendar + task + meal plan UI plugins

**Labels:** `type: plugin`, `phase: 4 — msgraph`

Build the UI plugins that display Microsoft Graph data.

**Acceptance Criteria:**
- `calendar-agenda`: 5-day rolling view, color-coded by calendar, shows all-day events as banners, time-based events as list items
- `task-list`: grouped by list, shows due dates, priority, completion status
- `meal-plan`: filters calendar events by a configurable keyword/category (e.g., "Meal Plan" calendar), displays as a weekly grid
- `clock` UI plugin: digital clock with date, configurable format (12/24h)
- All plugins respect theme tokens and are responsive within their grid cells

---

## Phase 5: Home Integration

### Issue #27 — Home Assistant data plugin

**Labels:** `type: plugin`, `phase: 5 — home`

Build the Home Assistant data plugin that connects via the HA REST API.

**Acceptance Criteria:**
- Manifest setup fields: HA URL (text), long-lived access token (password), entity filter (multi-select, populated after connection test)
- Fetches device states → `home_devices` shape
- Handles lights, locks, doors, garage, sensors, climate
- Groups by area/room as configured in HA
- Refresh interval: 30 seconds default (fast for door/lock state)

---

### Issue #28 — Home + infrastructure + media + package UI plugins

**Labels:** `type: plugin`, `phase: 5 — home`

Build the remaining UI plugins for home and infrastructure data.

**Acceptance Criteria:**
- `home-status`: device grid with icons, on/off state, grouped by room. Color-coded status (green = locked/closed/off, yellow = on, red = open/unlocked)
- `server-health`: server/service list with status indicators (up/down/degraded), CPU/memory/disk bars
- `media-now-playing`: album art, track name, artist, playback state. Shows "Nothing playing" when idle
- `package-tracker`: delivery list with carrier, status, ETA. Visual progress bar for transit state

---

## Phase 6: Client Support (Server-Side)

### Issue #29 — Client registration + management API

**Labels:** `type: api`, `phase: 6 — clients`

Build the server-side support for dedicated client apps: registration, polling, offline screen, and admin management.

**Acceptance Criteria:**
- `clients` table migration (client_id, name, display_id, status, last_seen_at, offline_mode, platform, app_version)
- `POST /api/clients/register` — client sends name + generated client_id, gets `pending` status
- `GET /api/clients/{client_id}/config` — returns assigned display slug, status, poll interval (or "pending" if not yet approved)
- `GET /api/clients/{client_id}/offline` — returns pre-rendered offline screen HTML
- Admin API: list clients, approve/reject, assign to display, view last-seen
- Offline screen config stored per-display (admin designs it in display settings)

---

### Issue #30 — Admin UI: client management + offline screen config

**Labels:** `type: frontend`, `phase: 6 — clients`

Build the admin pages for managing connected client devices and configuring offline screens.

**Acceptance Criteria:**
- **Client list page:** shows all registered clients with status (pending/approved/offline), platform, last seen, assigned display
- **Approve/reject flow:** pending clients show with a pairing code, admin approves and assigns to a display
- **Per-display offline screen config** in the display designer: choose offline mode (show offline screen / screen off via CEC / show last screenshot), design the offline screen content (title, message, basic status)
- **Pi setup script** (`setup-kiosk.sh`): auto-configures Chromium kiosk mode for raw-browser users on Raspberry Pi

---

## Phase 7: Polish

### Issue #31 — Bundled themes + responsive adjustments

**Labels:** `type: frontend`, `phase: 7 — polish`

Create bundled theme presets and handle non-standard display aspect ratios.

**Acceptance Criteria:**
- Bundled themes: glass dark, solid dark, glass light, solid light (at minimum)
- Themes selectable from a gallery in the theme editor
- Weather alerts rendering in the bottom bar
- Grid engine handles non-16:9 displays gracefully (portrait tablets, ultrawide monitors)
- Performance tuning for Raspberry Pi: minimize re-renders, lazy-load off-screen cards, optimize bundle size

---

### Issue #32 — Plugin development documentation

**Labels:** `type: docs`, `phase: 7 — polish`

Write developer guides for extending the dashboard with custom plugins.

**Acceptance Criteria:**
- Guide: writing a custom data plugin (Go, implementing the interface, manifest, registration)
- Guide: writing a custom UI plugin (React component, `configSchema`, consuming data shapes)
- Guide: adding a new data shape
- Example plugin templates (one data, one UI) with inline comments
- Contributing guide with code style, PR process, testing requirements

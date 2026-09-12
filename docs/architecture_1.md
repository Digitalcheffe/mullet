# Architecture Document

> Build blueprint for the self-hosted widget dashboard framework.
> Audience: Ryan + Claude (coding reference). Updated as decisions are made.

---

## Overview

A self-hosted, open-source widget dashboard designed for wall-mounted displays (Raspberry Pi + monitor, old tablet, any kiosk browser). What Home Assistant did for smart home, but for info displays. No subscriptions, no vendor lock-in, no cloud dependency.

**Two-piece system:**
- **Server** (Docker container): Go binary that runs data plugins on a schedule, writes to SQLite, serves a REST API, hosts the React frontend (display views + admin UI).
- **Client** (endpoint device): Either the dedicated client app (recommended) or any browser in kiosk mode. Each physical display gets its own URL: `http://server:8080/display/kitchen`, `http://server:8080/display/office`.

The server does all the work. The client just renders. One server, many displays. See the **Client** section for the two connection scenarios.

---

## Plugin System

The plugin architecture has three layers with strict ownership:

```
+------------------------------------------------------+
|                    FRAMEWORK                         |
|                                                      |
|   Owns:   Data Shapes (contracts)                    |
|           Grid System                                |
|           Plugin Interfaces                          |
|           SQLite schema                              |
|           REST API                                   |
|           Admin UI                                   |
+------------+----------------------------+------------+
             |                            |
     +-------v-------+          +--------v-------+
     |  DATA PLUGINS  |          |  UI PLUGINS    |
     |                |          |                |
     |  Fetch from    |          |  Read from     |
     |  external      |  --DB--> |  data shape,   |
     |  sources,      |          |  render in     |
     |  write to      |          |  grid card     |
     |  data shape    |          |                |
     +----------------+          +----------------+
```

**Key principle:** The framework defines data shapes. Data plugins write to them. UI plugins read from them. Neither plugin type owns the shapes. This means you can swap Microsoft Calendar for Google Calendar and the UI plugin doesn't care --- it just reads `events`. You can have two data plugins writing to the same shape (Microsoft Calendar + ICS feed both feeding `events`). You can have two UI plugins reading the same shape (agenda view + month view both consuming `events`).

### Compiled-In Plugins

All plugins ship compiled into the Go binary (data plugins) or bundled in the Vite build (UI plugins). There is no dynamic plugin loading at runtime.

- **Enabling/configuring** a plugin is hot. Done through the admin UI, stored in SQLite, no restart needed.
- **Adding a new plugin** means updating the Docker image (new image tag with the plugin compiled in) and restarting the container. This is standard Docker workflow --- `docker-compose pull && docker-compose up -d`.

This keeps the system simple. No plugin marketplace, no runtime module loading, no security surface from loading untrusted code. Community plugins are compiled into custom builds or submitted as PRs upstream.

---

## Data Contracts

Data contracts are Go types defined in the framework's `shapes` package. They are the compile-time contract between data plugins and UI plugins. A data plugin imports a contract type and returns typed data. A UI plugin declares which contract it consumes and receives typed data through the API.

**Two lanes:**

1. **Use a framework contract.** Plugin imports `shapes.Event`, returns `[]shapes.Event`. Any UI plugin consuming the `events` contract can display data from any data plugin that writes to it. Google Calendar + Microsoft Calendar both write `events`, and any events UI plugin can read from either interchangeably.

2. **Define a custom contract.** Plugin defines its own Go struct and registers it. A UI plugin consuming that custom contract ONLY gets data from plugins that write to it. No blending with framework contracts.

The rule: **contract type is the binding.** If a plugin author wants interoperability with the ecosystem, they use a framework contract. If they want full control, they go custom, but they're on their own island.

### Framework Contracts (Go Types)

Contracts are code objects in the `shapes` package. Plugin developers import them and code against them. The database stores the data in typed columns; the Go struct defines those columns.

```go
package shapes

import "time"

// Event represents a calendar event. Written by calendar data plugins,
// consumed by calendar/agenda UI plugins.
// Related metadata: calendars table (name, color, enabled)
type Event struct {
    ID           string     `db:"id"`
    CalendarID   int        `db:"calendar_id"`    // FK to calendars table
    Title        string     `db:"title"`
    Start        time.Time  `db:"start"`
    End          *time.Time `db:"end"`             // nil for open-ended all-day events
    AllDay       bool       `db:"all_day"`
    Location     *string    `db:"location"`
    Description  *string    `db:"description"`
}

// Task represents a to-do item. Written by task data plugins,
// consumed by task list UI plugins.
// Related metadata: task_lists table (name, enabled)
type Task struct {
    ID         string  `db:"id"`
    TaskListID int     `db:"task_list_id"`    // FK to task_lists table
    Title      string  `db:"title"`
    Completed  bool    `db:"completed"`
    DueDate    *string `db:"due_date"`        // date string, nullable
    SortOrder  int     `db:"sort_order"`
}

// WeatherCurrent represents current weather conditions.
// No related metadata table -- standalone data.
type WeatherCurrent struct {
    ID        string   `db:"id"`              // typically "current" (one row per plugin instance)
    Temp      float64  `db:"temp"`
    FeelsLike *float64 `db:"feels_like"`
    Condition string   `db:"condition"`       // "Partly Cloudy"
    Icon      string   `db:"icon"`            // icon key or emoji
    Humidity  *int     `db:"humidity"`        // percentage
    High      *float64 `db:"high"`            // today's high
    Low       *float64 `db:"low"`             // today's low
    Sunrise   *string  `db:"sunrise"`
    Sunset    *string  `db:"sunset"`
}

// WeatherForecast represents a single day's forecast.
// No related metadata table -- standalone data.
type WeatherForecast struct {
    ID           string   `db:"id"`            // typically the date string
    Date         string   `db:"date"`
    High         float64  `db:"high"`
    Low          float64  `db:"low"`
    Condition    string   `db:"condition"`
    Icon         string   `db:"icon"`
    PrecipChance *int     `db:"precip_chance"` // 0-100
}
```

Each contract also declares its write strategy and retention:

```go
// ShapeMeta defines how the framework handles data for this contract.
// Registered alongside the struct in the shape registry.
type ShapeMeta struct {
    Name           string         // "events", "tasks", "weather_current"
    WriteStrategy  WriteStrategy  // Replace or Upsert
    Retention      RetentionPolicy
    HasMetadata    bool           // true if this shape has a related metadata table
    MetadataTable  string         // "calendars", "task_lists" (empty if HasMetadata is false)
}

type WriteStrategy int
const (
    // Replace: delete all rows for the scoped key, insert new set.
    // Good for weather, media -- full snapshot each fetch.
    Replace WriteStrategy = iota

    // Upsert: compare by ID within scope, insert new / update changed / delete missing.
    // Good for events, tasks -- row identity matters.
    Upsert
)

type RetentionPolicy struct {
    Strategy   string // "window" | "latest" | "count"
    WindowDays int    // for "window": keep rows within N days of now
    MaxRows    int    // for "count": keep N most recent per plugin instance
    // "latest": only the most recent fetch survives (replace handles this implicitly)
}
```

V1 contract registration:

```go
var Events = ShapeMeta{
    Name:          "events",
    WriteStrategy: Upsert,
    Retention:     RetentionPolicy{Strategy: "window", WindowDays: 60},
    HasMetadata:   true,
    MetadataTable: "calendars",
}

var Tasks = ShapeMeta{
    Name:          "tasks",
    WriteStrategy: Upsert,
    Retention:     RetentionPolicy{Strategy: "latest"},
    HasMetadata:   true,
    MetadataTable: "task_lists",
}

var WeatherCurrentMeta = ShapeMeta{
    Name:          "weather_current",
    WriteStrategy: Replace,
    Retention:     RetentionPolicy{Strategy: "latest"},
    HasMetadata:   false,
}

var WeatherForecastMeta = ShapeMeta{
    Name:          "weather_forecast",
    WriteStrategy: Replace,
    Retention:     RetentionPolicy{Strategy: "latest"},
    HasMetadata:   false,
}
```

### Metadata Tables

Contracts that reference a parent entity (calendars for events, task lists for tasks) store that metadata in its own table. This keeps entity info (name, color, enabled/disabled) out of every data row and gives the admin UI something to query directly.

When a data plugin authorizes and connects to a source, it discovers the available entities (calendars, task lists) and populates the metadata table. The admin UI shows the user which entities are available and lets them toggle which ones to sync.

**Calendars** (used by events contract):

```sql
CREATE TABLE calendars (
    id                 INTEGER PRIMARY KEY,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,            -- calendar ID from the source API
    name               TEXT NOT NULL,            -- "Work", "Family", "Meal Plan"
    color              TEXT,                     -- hex color from source
    enabled            INTEGER NOT NULL DEFAULT 1,
    UNIQUE(plugin_instance_id, external_id)
);
```

**Task Lists** (used by tasks contract):

```sql
CREATE TABLE task_lists (
    id                 INTEGER PRIMARY KEY,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    name               TEXT NOT NULL,            -- "Chores", "Groceries", "Homework"
    enabled            INTEGER NOT NULL DEFAULT 1,
    UNIQUE(plugin_instance_id, external_id)
);
```

The flow for calendar setup:

1. User adds MS Calendar plugin instance, completes OAuth
2. Plugin queries Graph API for user's calendars
3. Plugin writes discovered calendars to the `calendars` table
4. Admin UI shows: "Which calendars do you want on your display?" with toggles per calendar
5. User enables Work, Family, Meal Plan. Disables Holidays.
6. Scheduler runs the plugin -- plugin reads `calendars` table for enabled ones, fetches events for only those calendars
7. Events written to `shape_events` with `calendar_id` foreign key
8. UI plugin queries events, joins on calendars for name/color

### Custom Contracts

A plugin author who needs a shape that doesn't match any framework contract defines their own Go struct, registers it with a `ShapeMeta`, and provides a migration for its data table. The framework creates the table on startup. A custom contract's data table follows the same pattern as framework tables (typed columns, `plugin_instance_id`, `fetched_at`), and the custom plugin can optionally include its own metadata table.

Custom contracts cannot share a name with a framework contract. A UI plugin consuming a custom contract only connects to data plugins writing to that exact contract. No cross-pollination.

### Write Path

When the scheduler triggers a data plugin fetch:

1. Plugin checks its metadata table for enabled entities (e.g., enabled calendars)
2. Plugin fetches from external API, scoped to enabled entities
3. Plugin returns typed data: `[]shapes.Event` (or its custom struct)
4. Framework applies the write strategy, scoped by `plugin_instance_id` and metadata key (e.g., `calendar_id`):
   - **Replace**: `DELETE FROM shape_events WHERE plugin_instance_id = ? AND calendar_id = ?` then bulk `INSERT`
   - **Upsert**: `DELETE` + `INSERT` within scope (same effect, simpler than row-by-row comparison)
5. Framework logs fetch result (success/error, row count, timestamp) to `data_plugin_instances`

The write is always scoped. Updating "Work" calendar events never touches "Family" calendar events. No JSON parsing, no in-memory set comparison. Clean SQL.

### Retention / Cleanup

A cleanup job runs periodically (piggybacks on the scheduler loop). For each contract:

- **`latest`**: handled implicitly by the Replace write strategy (old data deleted on each fetch)
- **`window`**: `DELETE FROM shape_events WHERE start < datetime('now', '-60 days')` (events contract uses the `start` field; the ShapeMeta could specify which field drives the window)
- **`count`**: `DELETE` oldest rows beyond the limit per plugin instance

### API Rate Limit Awareness

API call limits are a data plugin concern, not a contract concern. The plugin manifest includes recommended and minimum fetch intervals:

```go
type DataPluginManifest struct {
    // ...existing fields...
    RecommendedInterval time.Duration  // suggested polling frequency
    MinInterval         time.Duration  // hard floor (prevents burning API quota)
}
```

The admin UI uses these as guardrails when the user configures polling frequency. If someone sets OpenWeatherMap to refresh every 10 seconds, the UI warns them based on `MinInterval`.

---

## Data Plugins

A data plugin is a Go package that implements the `DataPlugin` interface and declares its configuration needs via a manifest. The manifest drives the admin UI setup wizard -- no hardcoded forms per plugin.

### Plugin Manifest

```go
type DataPluginManifest struct {
    ID                  string         // "msgraph-calendar"
    Name                string         // "Microsoft Calendar"
    Description         string         // "Syncs calendars from Outlook / Microsoft 365"
    Contract            string         // "events" (framework contract name or custom contract name)
    SetupFields         []SetupField   // drives the admin wizard form
    AuthType            string         // "none", "api_key", "oauth2"
    OAuthConfig         *OAuthConfig   // only if AuthType == "oauth2"
    RecommendedInterval time.Duration  // suggested polling frequency
    MinInterval         time.Duration  // hard floor for API rate limits
}

type SetupField struct {
    Key         string // "tenant_id"
    Label       string // "Tenant ID"
    Type        string // "text", "select", "multi-select", "number", "toggle", "password"
    Required    bool
    Default     any
    Placeholder string
    HelpText    string
    Options     []SelectOption // only for "select" and "multi-select"
}

type SelectOption struct {
    Value string
    Label string
}

type OAuthConfig struct {
    AuthURL     string
    TokenURL    string
    Scopes      []string
    TenantField string // which SetupField holds the tenant ID, if applicable
}
```

When a user adds a data plugin in the admin UI:
1. Admin UI reads the manifest's `SetupFields`
2. Renders a setup wizard with the appropriate form fields
3. If `AuthType == "oauth2"`, embeds the OAuth2 authorization flow
4. Completed config is stored as JSON in `data_plugin_instances`
5. Plugin connects and discovers available entities (calendars, task lists) -- populates metadata table
6. Admin UI shows discovered entities with enable/disable toggles
7. Plugin is enabled and the scheduler starts polling it

### Plugin Interface

```go
type DataPlugin interface {
    ID() string
    Name() string
    Manifest() DataPluginManifest
    Contract() string                                      // which data contract this writes to
    Configure(cfg map[string]any) error                    // apply config from setup wizard
    DiscoverEntities(ctx context.Context) error            // populate metadata table (calendars, task lists)
    Fetch(ctx context.Context) ([]any, error)              // returns typed contract rows (e.g., []shapes.Event)
    RefreshInterval() time.Duration
}
```

Key changes from earlier design:

- `Fetch()` returns typed data (`[]shapes.Event`, `[]shapes.WeatherCurrent`, etc.) cast as `[]any`. The framework knows the contract type and handles the typed write.
- `DiscoverEntities()` is a separate step from `Fetch()`. Called after initial config/auth to populate metadata tables. Can be re-called from the admin UI to refresh the entity list.
- `Contract()` replaces `DataShape()` to match the new terminology.

Each data plugin lives in its own package under `internal/plugins/data/`.

### V1 Data Plugins

| Plugin | Contract | Source | Auth |
|--------|----------|--------|------|
| `msgraph-calendar` | events | Microsoft Graph API | OAuth2 |
| `msgraph-todo` | tasks | Microsoft Graph API | OAuth2 |
| `openweathermap` | weather_current, weather_forecast | OpenWeatherMap API | API key |
| `clock` | (none, local) | System clock | None |

> Future data plugins: Google Calendar (events contract), Home Assistant (custom contract), ICS file (events contract), Portainer, PRTG, package tracking, media players. Community plugins can use framework contracts for interoperability or define custom contracts for specialized data.

---

## UI Plugins

A UI plugin is a React component that declares what data shape it consumes and how much grid space it needs.

```typescript
interface UIPlugin {
  id: string;                       // unique plugin identifier
  name: string;                     // human-readable
  dataShape: string;                // which shape this reads: "events", "tasks", etc.
  defaultSize: { w: number; h: number };  // default grid units
  minSize: { w: number; h: number };      // minimum grid units
  maxSize?: { w: number; h: number };     // optional maximum
  configSchema?: Record<string, ConfigField>;  // plugin-specific settings (rendered in admin UI)
  component: React.ComponentType<WidgetProps>;
}

interface ConfigField {
  type: 'text' | 'number' | 'select' | 'multi-select' | 'toggle' | 'color';
  label: string;
  default?: any;
  options?: { value: string; label: string }[];
  helpText?: string;
}

interface WidgetProps {
  data: any[];                    // rows from the declared data shape
  config: Record<string, any>;   // user config from admin UI (stored in SQLite)
  size: { w: number; h: number };// actual grid dimensions
  theme: ThemeContext;            // current theme tokens
}
```

Each UI plugin lives in its own folder under `web/src/plugins/`. The `configSchema` drives per-card settings in the Display Designer (e.g., how many days the calendar shows, which task list to filter to).

### V1 UI Plugins

| Plugin | Shape | Description |
|--------|-------|-------------|
| `calendar-agenda` | events | Multi-day rolling agenda with calendar color coding |
| `task-list` | tasks | Checklist with assignees and completion state |
| `weather-current` | weather_current | Current temp, condition, hi/lo, sunrise/sunset |
| `weather-forecast` | weather_forecast | 5-day forecast row |
| `meal-plan` | events | Week grid filtered to "Meal Plan" calendar |
| `home-status` | home_devices | Door/garage/sensor status list |
| `package-tracker` | packages | Incoming packages with ETAs |
| `server-health` | infrastructure | Server/container status dots |
| `media-now-playing` | media_status | TV/speaker status |
| `clock` | (local) | Time, date, no data shape needed |

---

## Display Hierarchy

One server supports multiple physical displays. Each display has its own URL and its own layout.

```
Display            "Kitchen"  --  http://server:8080/display/kitchen
  |
  +-- Screen 1     "Main"     --  auto-rotates (configurable interval)
  |     |
  |     +-- Card   col:1 row:1 w:4 h:10  -->  calendar-agenda (UI plugin)
  |     +-- Card   col:5 row:1 w:8 h:3   -->  weather-forecast (UI plugin)
  |     +-- Card   col:5 row:4 w:8 h:3   -->  meal-plan (UI plugin)
  |     +-- ...
  |
  +-- Screen 2     "Detail"   --  rotated to on interval or manual trigger
        |
        +-- Card   col:1 row:1 w:16 h:12 -->  full-screen calendar month view
```

### Hierarchy Rules

- **Display**: A named physical output (kitchen, office, bedroom). Has a slug for URL routing, a base theme, and grid dimensions.
- **Screen**: A page within a display. Screens rotate on a configurable interval with transition effects (fade, slide, none). A display must have at least one screen.
- **Card**: A positioned rectangle on the screen's grid. References a UI plugin and carries plugin-specific config (stored as JSON) plus an optional theme override.
- **UI Plugin**: The React component that renders inside a card. Reads data from its declared shape.

### Grid System

Each display has a configurable grid. Default is 16 columns x 12 rows (16:9 displays). Cards snap to grid cells. The grid has two fixed zones (top bar, bottom bar) and a main content area.

```
+----------------------------------------------------------+
| TOP BAR (clock/date left, weather right) -- fixed        |
+----------+-------------------------------+---------------+
|          |                               |               |
| Calendar |   Forecast                    |  Indoor Temp  |
| Agenda   |   Meal Plan                   |  Home Status  |
|          |   Chores                      |  Packages     |
| (4 cols) |   (8 cols)                    |  Infra Health |
|          |                               |  (4 cols)     |
|          |                               |               |
+----------+-------------------------------+---------------+
| BOTTOM BAR (now playing, alerts, screen dots) -- fixed   |
+----------------------------------------------------------+
```

---

## Client

Two ways to connect a display to the server. Both are supported; the dedicated client is the recommended path for permanent wall-mounted displays.

### Scenario 1: Dedicated Client App (Recommended)

**Separate project, separate repo.** The client has its own build pipeline and release cycle. What's documented here are the architectural decisions and the API contract between server and client. The client repo(s) will have their own internal docs.

A lightweight native app installed on the endpoint device. Handles registration, polling, offline fallback, and fullscreen rendering. No browser configuration needed.

#### Platforms

| Target | Device | Technology | Repo |
|--------|--------|------------|------|
| Linux ARM | Raspberry Pi | Go + webview (uses system WebKit) | `project-name-client` |
| Linux x86 | Old laptop/PC | Go + webview | `project-name-client` (same) |
| Android / Android TV | Android tablets, smart TVs with Android TV, older Fire TV Sticks | Kotlin + WebView | `project-name-android` |

The Go client and Android client are separate repos (different languages, different toolchains) but implement the same behavior against the same server API. The Go binary cross-compiles for ARM and x86 from one codebase. The Android APK covers Android tablets and any smart TV running Android TV.

> **Fire TV Stick Warning (2026):** Amazon's newer Fire TV devices (2nd gen Fire TV Stick HD, Fire TV Stick 4K Select, and all models going forward) run Vega OS, which blocks sideloading entirely. The product listing explicitly states: "this device prevents sideloading or installing apps from unknown sources." Older Fire TV Sticks on Fire OS 7/8 still support sideloading, but new stock is locked down. For cheap dedicated display hardware, a Raspberry Pi Zero 2 W (~$15) or an Onn 4K Streaming Stick (~$15-20, supports sideloading) are better recommendations. Publishing to the Amazon Appstore is a future option but adds review friction and update delays.

#### Registration Flow

1. User installs the client on the endpoint device
2. Opens the client, enters the server URL and a display name (e.g., "Kitchen")
3. Client generates a random client ID and displays it on screen (large, centered, like a pairing code)
4. Admin opens the server's admin UI, sees the pending client in the client management page
5. Admin approves the client and assigns it to a display configuration
6. Client receives its display assignment on the next poll and starts rendering

```
+------------------------------------------+
|                                          |
|         Waiting for Approval             |
|                                          |
|     Server: http://192.168.1.50:8080     |
|     Name:   Kitchen                      |
|                                          |
|          Client ID: X7K-M2P              |
|                                          |
|   Approve this client in the admin UI    |
|                                          |
+------------------------------------------+
```

Once approved, the assignment persists. If the client reboots, it already knows its server URL, client ID, and display assignment. No re-pairing needed.

#### Polling Cycle

The client does not maintain a persistent connection. It polls.

1. Client hits the server on a configurable interval (default: 15 seconds)
2. Server responds with the display page (or a "no changes" signal if nothing changed)
3. Client reloads/refreshes the rendered page
4. If the poll fails (server unreachable, timeout, error): enter offline mode
5. Continue polling on a backoff interval (e.g., 15s, 30s, 60s) until server returns

#### Offline Mode

When the server is unreachable, the client shows a server-configured offline screen. This is NOT a cached copy of the live dashboard. It is a pre-rendered, self-contained HTML page designed by the admin and pulled from the server during normal operation.

**How it works:**

1. During normal operation, the client periodically pulls the offline screen from the server and caches it locally:
   ```
   GET /api/clients/{client_id}/offline
   Response: { "html": "<self-contained HTML page>", "updated_at": "..." }
   ```
2. The server pre-renders this page with all styles inlined, no external dependencies. It could be:
   - A simple clock + display name (auto-generated default, no admin config needed)
   - A custom offline page the admin designed (static image, message, family photo, etc.)
   - Configured per-display in the admin UI
3. When the server goes down, the client switches to this cached offline screen
4. The offline screen is themed to match the display's theme (so it doesn't look like a different app)
5. A subtle indicator shows the display is offline (small amber dot, "offline" label, etc.)
6. Client continues polling. When the server returns, it switches back to the live display.

**Alternative offline behavior** (configurable per client):
- `offline_screen` (default): show the cached offline page
- `screen_off`: put the display to sleep / turn off via HDMI-CEC
- `last_screenshot`: show a screenshot of the last live frame (frozen, no updates)

```
+------------------------------------------+
|                              [offline]   |
|                                          |
|              12:47 PM                    |
|         Sunday, Sep 6, 2026              |
|                                          |
|             Kitchen Display              |
|                                          |
|       Waiting for server connection      |
|                                          |
+------------------------------------------+
```

#### Client Config

The client stores minimal local config (not on the server):

```
server_url: http://192.168.1.50:8080
client_id: x7k-m2p-abc123
display_name: Kitchen
poll_interval: 15          # seconds
offline_mode: offline_screen   # offline_screen | screen_off | last_screenshot
```

Everything else (what to display, theme, layout) comes from the server. The client is intentionally thin.

#### HDMI-CEC (Raspberry Pi)

On Pi hardware connected via HDMI, the client can control the TV/monitor:

- Screen schedule: turn display off at 11 PM, on at 6 AM
- Offline mode `screen_off`: turn display off when server is unreachable
- Source switching: auto-switch the TV to the Pi's HDMI input on boot

Uses `cec-client` (libCEC), which is standard on Raspbian. Not available on Android TV devices (those devices ARE the TV's input).

#### Server-Side: Client Management

The server tracks registered clients. New admin UI and API additions:

**SQLite table:**

```sql
CREATE TABLE clients (
    id              INTEGER PRIMARY KEY,
    client_id       TEXT UNIQUE NOT NULL,     -- random pairing ID
    name            TEXT NOT NULL,            -- "Kitchen" (user-given)
    display_id      INTEGER REFERENCES displays(id),  -- assigned display, nullable until approved
    status          TEXT NOT NULL DEFAULT 'pending',   -- pending, approved, offline
    last_seen_at    DATETIME,
    offline_mode    TEXT NOT NULL DEFAULT 'offline_screen',
    platform        TEXT,                     -- "linux-arm", "linux-x86", "android"
    app_version     TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Admin UI additions (Zone 1):**
- Client management page: list registered clients, approve pending clients, assign/reassign to displays, see online/offline status and last-seen time
- Per-display offline screen config in the Display Designer

**API additions:**

```
# Client registration and polling (no auth -- clients are pre-approved by the admin)
POST /api/clients/register             # Client sends: { name, client_id, platform, version }
GET  /api/clients/{client_id}/config   # Client polls: returns display assignment + status
GET  /api/clients/{client_id}/offline  # Client pulls offline screen HTML

# Admin client management (requires auth)
GET    /api/admin/clients              # List all clients
PUT    /api/admin/clients/{id}/approve # Approve + assign to display
PUT    /api/admin/clients/{id}         # Update client (reassign display, change offline mode)
DELETE /api/admin/clients/{id}         # Remove client
```

### Scenario 2: Raw Browser (No Client App)

For devices where installing the client isn't practical (iPad on the counter, a smart TV's built-in browser, a kiosk PC that already has Chrome), the server still serves the display as a plain web page.

**URL:** `http://server:8080/display/{slug}`

The browser loads the React display app directly. No registration, no pairing code. The admin creates the display in the Display Designer and gives the user the URL to open.

> **Note:** `/register` (issue #81) since shipped a browser-based stand-in for Scenario 1's pairing flow -- open it on the screen's device, give it a name, and approve the resulting pairing code from the admin's Clients page, same as a dedicated client app would. This section still applies as-is to anyone who'd rather skip pairing entirely and open a display's URL directly.

#### What You Get

- Full display rendering (grid, cards, themes, screen rotation)
- Live data updates via the React app's own polling (fetches data shapes from the API)
- Works on any modern browser

#### What You Don't Get

- No offline fallback (browser shows its own error page if server is down)
- No HDMI-CEC / screen power management
- No auto-start on boot (user must configure kiosk mode themselves)
- No auto-discovery of the server

#### Kiosk Mode Setup (Per Platform)

For users going the raw browser route, we provide setup scripts / docs:

- **Raspberry Pi + Chromium:** `setup-kiosk.sh` script that configures Chromium to auto-start in kiosk mode, hides the cursor, disables screensaver, auto-refreshes on crash
- **iPad:** Open Safari, add to home screen, enable Guided Access
- **Smart TV browser:** Just open the URL. No kiosk mode available. Display may show browser chrome.
- **Old laptop + Chrome:** Chrome `--kiosk` flag, auto-start on login

#### Display-Side Resilience (Built Into React App)

Even without the dedicated client, the React display app handles connection drops gracefully:

- If the API poll fails, the React app shows a built-in "reconnecting" overlay on top of the last rendered frame
- The last successfully rendered UI stays visible behind the overlay (it's still in the DOM, just frozen)
- Polling continues with backoff. When the server returns, overlay dismisses, data refreshes.
- This is NOT the same as the client's offline screen (no server-designed fallback, no cached image). It's just the React app being polite about connection drops rather than showing a blank screen.

### Client Comparison

| Feature | Dedicated Client | Raw Browser |
|---------|-----------------|-------------|
| Setup | Install app, enter URL, approve in admin | Open URL, configure kiosk mode manually |
| Registration | Pairing code, admin approval | None (direct URL) |
| Offline fallback | Server-designed offline screen | "Reconnecting" overlay on frozen UI |
| Screen power | HDMI-CEC schedule (Pi), screen_off mode | Not available |
| Auto-start on boot | Built in | Manual config per OS |
| Platform | Pi, Linux PC, Android TV, older Fire TV Sticks | Any browser |
| Recommended for | Permanent wall-mounted displays | Temporary/casual displays, tablets |

---

## SQLite Schema

SQLite is the single source of truth for all configuration, layout, and cached data. No YAML. No config files for end users. Everything is managed through the admin UI.

### System Tables

```sql
-- Server-level settings (port, auth, etc.)
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- User accounts for admin UI access
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin',  -- admin | viewer (future)
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Plugin Configuration Tables

```sql
-- Enabled data plugins and their configuration
CREATE TABLE data_plugin_instances (
    id              INTEGER PRIMARY KEY,
    plugin_id       TEXT NOT NULL,           -- "msgraph-calendar"
    instance_name   TEXT NOT NULL,           -- "Work Calendar" (user-given name)
    config          TEXT NOT NULL DEFAULT '{}',  -- JSON blob from setup wizard
    enabled         INTEGER NOT NULL DEFAULT 1,
    refresh_seconds INTEGER NOT NULL,        -- polling interval
    last_fetch_at   DATETIME,
    last_error      TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- OAuth2 tokens (used by plugins with AuthType "oauth2")
CREATE TABLE oauth_tokens (
    id                    INTEGER PRIMARY KEY,
    plugin_instance_id    INTEGER NOT NULL REFERENCES data_plugin_instances(id),
    access_token          TEXT NOT NULL,
    refresh_token         TEXT,
    token_type            TEXT NOT NULL DEFAULT 'Bearer',
    expiry                DATETIME,
    created_at            DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Display Layout Tables

```sql
-- Physical displays
CREATE TABLE displays (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,              -- "Kitchen"
    slug        TEXT UNIQUE NOT NULL,       -- "kitchen" (URL path)
    grid_cols   INTEGER NOT NULL DEFAULT 16,
    grid_rows   INTEGER NOT NULL DEFAULT 12,
    theme_id    INTEGER REFERENCES themes(id),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Screens within a display (ordered, rotatable)
CREATE TABLE screens (
    id                    INTEGER PRIMARY KEY,
    display_id            INTEGER NOT NULL REFERENCES displays(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,          -- "Main", "Detail"
    sort_order            INTEGER NOT NULL DEFAULT 0,
    transition_type       TEXT NOT NULL DEFAULT 'fade',  -- fade, slide, none
    transition_interval   INTEGER NOT NULL DEFAULT 30,   -- seconds, 0 = manual only
    created_at            DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Cards placed on a screen's grid
CREATE TABLE cards (
    id              INTEGER PRIMARY KEY,
    screen_id       INTEGER NOT NULL REFERENCES screens(id) ON DELETE CASCADE,
    ui_plugin_id    TEXT NOT NULL,            -- "calendar-agenda"
    col             INTEGER NOT NULL,         -- grid column (1-based)
    row             INTEGER NOT NULL,         -- grid row (1-based)
    width           INTEGER NOT NULL,         -- grid units wide
    height          INTEGER NOT NULL,         -- grid units tall
    plugin_config   TEXT NOT NULL DEFAULT '{}',  -- JSON: per-card UI plugin settings
    theme_override  TEXT,                     -- JSON: nullable, card-level theme overrides
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Theme Tables

```sql
CREATE TABLE themes (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,                -- "Glass Dark", "Solid Light"
    tokens      TEXT NOT NULL DEFAULT '{}',   -- JSON: full theme token set
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Theme tokens include: background (type, value), card background, card border, text colors, accent color, font family, font sizes, border radius, opacity, blur amount, etc. The Display Designer's theme panel edits these visually.

### Theme Cascade

1. **Display** sets the base theme (references `themes` table)
2. **Cards** inherit the display theme by default
3. **Cards** can override specific tokens via `theme_override` JSON (nullable --- null means full inheritance)

Theme overrides will be limited to prevent visual chaos (e.g., card can change its own background opacity and accent color, but not the global font or layout spacing).

### Cached Data Tables

Each framework contract gets a typed `shape_{name}` table with real columns matching its Go struct. No JSON blobs — every field is a queryable column. The framework creates these on startup based on the registered contracts. Custom contracts provide their own migrations.

**Events** (contract: `shapes.Event`):

```sql
CREATE TABLE shape_events (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    calendar_id        INTEGER NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    start              DATETIME NOT NULL,
    end                DATETIME,
    all_day            INTEGER NOT NULL DEFAULT 0,
    location           TEXT,
    description        TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_events_calendar ON shape_events(calendar_id);
CREATE INDEX idx_events_start ON shape_events(start);
```

**Tasks** (contract: `shapes.Task`):

```sql
CREATE TABLE shape_tasks (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    task_list_id       INTEGER NOT NULL REFERENCES task_lists(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    completed          INTEGER NOT NULL DEFAULT 0,
    due_date           TEXT,              -- date string, nullable
    sort_order         INTEGER NOT NULL DEFAULT 0,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_tasks_list ON shape_tasks(task_list_id);
```

**Weather Current** (contract: `shapes.WeatherCurrent`):

```sql
CREATE TABLE shape_weather_current (
    id                 TEXT NOT NULL,     -- typically "current"
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    temp               REAL NOT NULL,
    feels_like         REAL,
    condition          TEXT NOT NULL,     -- "Partly Cloudy"
    icon               TEXT NOT NULL,     -- icon key or emoji
    humidity           INTEGER,           -- percentage
    high               REAL,             -- today's high
    low                REAL,             -- today's low
    sunrise            TEXT,
    sunset             TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
```

**Weather Forecast** (contract: `shapes.WeatherForecast`):

```sql
CREATE TABLE shape_weather_forecast (
    id                 TEXT NOT NULL,     -- typically the date string
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    date               TEXT NOT NULL,
    high               REAL NOT NULL,
    low                REAL NOT NULL,
    condition          TEXT NOT NULL,
    icon               TEXT NOT NULL,
    precip_chance      INTEGER,           -- 0-100
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_forecast_date ON shape_weather_forecast(date);
```

Every table includes `plugin_instance_id` for scoped writes and `fetched_at` for retention cleanup. Tables with metadata foreign keys (`calendar_id`, `task_list_id`) cascade on delete — removing a calendar from the metadata table automatically cleans up its events. The metadata tables themselves (`calendars`, `task_lists`) are defined in the Data Contracts section above.

---

## Admin UI

The admin UI is a React SPA served at `/admin`. It is the only way to configure the system (no YAML, no config files, no CLI flags for layout). Three zones:

### Zone 1: System Admin

**Route:** `/admin`

- First-run setup wizard (create admin account, set server name)
- User management (create/edit admin accounts)
- System settings (server port, timezone, etc.)
- Server health / logs

### Zone 2: Data Plugin Management

**Route:** `/admin/plugins`

- Browse available data plugins (compiled into the binary)
- Add a new data plugin instance:
  1. Pick plugin from the list (shows name, description, data shape)
  2. Auto-generated setup wizard from the plugin's manifest `SetupFields`
  3. If OAuth2: embedded authorization flow (redirect to provider, callback stores token)
  4. If API key: password-masked input field
  5. Name the instance (e.g., "Work Calendar", "Personal Calendar")
  6. Set polling interval (plugin provides default, user can override)
  7. Save --- config written to `data_plugin_instances`, scheduler picks it up immediately
- Edit/disable/delete existing plugin instances
- Status dashboard: last fetch time, errors, row counts per instance

### Zone 3: Display Designer

**Route:** `/admin/displays`

This is the drag-and-drop layout editor. The core experience of the admin UI.

- **Display list**: Create/edit/delete displays. Each display has a name, slug (URL), grid dimensions, and theme.
- **Screen manager**: Add/remove/reorder screens within a display. Set transition type and interval.
- **Grid editor** (the drag-and-drop surface):
  - Visual grid matching the display's dimensions
  - Left sidebar: available UI plugins (draggable)
  - Drag a UI plugin onto the grid to create a card
  - Resize cards by dragging edges/corners (respects min/max from UI plugin)
  - Move cards by dragging
  - Click a card to open its settings panel:
    - UI plugin config (rendered from the plugin's `configSchema`)
    - Theme override (optional: card-level style tweaks)
    - Delete card
  - Live preview: the grid editor shows a real-time-ish preview of how the display will look
- **Theme editor**: Edit the display's theme tokens visually (background, colors, fonts, card styles). Preview updates in real-time on the grid editor.

The display designer stores everything to the `displays`, `screens`, and `cards` tables. The actual display endpoint (`/display/{slug}`) reads from these tables and renders.

---

## Data Flow

```
External Source       Data Plugin          SQLite              API              UI Plugin
     |                    |                  |                  |                  |
     |  poll on interval  |                  |                  |                  |
     |<-------------------|                  |                  |                  |
     |  raw response      |                  |                  |                  |
     |------------------->|                  |                  |                  |
     |                    |  normalize to    |                  |                  |
     |                    |  data shape,     |                  |                  |
     |                    |  write rows      |                  |                  |
     |                    |----------------->|                  |                  |
     |                    |                  |                  |                  |
     |                    |                  | GET /api/data/   |                  |
     |                    |                  | shapes/events    |                  |
     |                    |                  |<-----------------|                  |
     |                    |                  | JSON rows        |                  |
     |                    |                  |----------------->|                  |
     |                    |                  |                  | typed data       |
     |                    |                  |                  |----------------->|
     |                    |                  |                  | render in card   |
```

**Polling intervals (configurable per plugin instance, defaults):**
- Calendar/tasks: 5 minutes
- Weather: 15 minutes
- Home devices: 30 seconds
- Infrastructure: 60 seconds
- Media status: 10 seconds

---

## API Endpoints

### Data API (consumed by display frontend)

```
GET  /api/data/shapes                  # List available data shapes
GET  /api/data/shapes/{shape}          # Get rows for a shape (e.g., /api/data/shapes/events)
                                       #   Query params: ?calendar_name=Personal&event_type=birthday
GET  /api/displays/{slug}              # Display config (grid, screens, cards, theme) for rendering
GET  /api/health                       # Server health check
```

### Admin API (consumed by admin UI, requires auth)

```
# Auth
POST /api/admin/auth/login             # Login, returns JWT
POST /api/admin/auth/setup             # First-run: create initial admin account

# System
GET  /api/admin/settings               # Get system settings
PUT  /api/admin/settings               # Update system settings

# Data Plugins
GET  /api/admin/plugins/data           # List available data plugins (from registry)
GET  /api/admin/plugins/data/instances # List configured plugin instances
POST /api/admin/plugins/data/instances # Create a new plugin instance
PUT  /api/admin/plugins/data/instances/{id}  # Update instance config
DELETE /api/admin/plugins/data/instances/{id} # Remove instance

GET  /api/admin/plugins/data/{id}/manifest   # Get plugin manifest (for wizard rendering)
GET  /api/admin/plugins/data/instances/{id}/status  # Fetch status, last error, row count

# OAuth2 (triggered during plugin setup)
GET  /api/admin/oauth/{plugin_instance_id}/authorize   # Start OAuth2 flow
GET  /api/admin/oauth/callback                         # OAuth2 callback (stores tokens)

# UI Plugins
GET  /api/admin/plugins/ui             # List available UI plugins

# Displays
GET  /api/admin/displays               # List all displays
POST /api/admin/displays               # Create display
PUT  /api/admin/displays/{id}          # Update display (name, slug, grid, theme)
DELETE /api/admin/displays/{id}        # Delete display

# Screens
GET  /api/admin/displays/{id}/screens          # List screens for a display
POST /api/admin/displays/{id}/screens          # Create screen
PUT  /api/admin/screens/{id}                   # Update screen
DELETE /api/admin/screens/{id}                 # Delete screen
PUT  /api/admin/displays/{id}/screens/reorder  # Reorder screens

# Cards
GET  /api/admin/screens/{id}/cards     # List cards on a screen
POST /api/admin/screens/{id}/cards     # Create card (place UI plugin on grid)
PUT  /api/admin/cards/{id}             # Update card (move, resize, config, theme override)
DELETE /api/admin/cards/{id}           # Remove card

# Themes
GET  /api/admin/themes                 # List themes
POST /api/admin/themes                 # Create theme
PUT  /api/admin/themes/{id}            # Update theme tokens
DELETE /api/admin/themes/{id}          # Delete theme (if not in use)
```

### Client API (consumed by dedicated client app, no auth -- clients are admin-approved)

```
POST /api/clients/register             # Register new client: { name, client_id, platform, version }
GET  /api/clients/{client_id}/config   # Poll for display assignment + status
GET  /api/clients/{client_id}/offline  # Pull offline screen HTML (cached locally by client)
```

### Display Endpoints (what the wall-mounted browser or client app hits)

```
GET  /display/{slug}                   # Renders the full-screen display view
                                       # Serves the React app configured for this display
```

---

## OAuth2 (Microsoft Graph)

Primary data layer for calendar and tasks. The manifest system drives the setup UI, but the OAuth2 flow deserves its own section because it's the most complex auth pattern.

Two flows depending on account type:

**Consumer (Personal Microsoft Account):**
- Register app in Azure Portal under "Personal Microsoft accounts only"
- Tenant ID stored in plugin config: `consumers`
- Standard OAuth2 authorization code flow
- User clicks "Authorize" in the data plugin setup wizard
- Redirected to Microsoft login, grants permissions
- Callback stores tokens in `oauth_tokens` table
- Tokens auto-refreshed by the framework before expiry

**Enterprise (Entra ID / Azure AD):**
- Register app in Azure Portal under the organization's tenant
- Tenant ID stored in plugin config: the organization's Azure tenant GUID
- May require admin consent for Graph scopes
- Same OAuth2 flow, different token endpoints

The framework handles both transparently based on the `tenant_id` in the plugin instance's config (which came from a `SetupField` in the manifest).

**Secrets handling:** OAuth2 client secrets and API keys are stored in SQLite (encrypted at rest is a future enhancement). Environment variables can be referenced in setup fields using `${VAR}` syntax for users who prefer not to enter secrets in the UI.

---

## Tech Stack

| Layer | Choice | Why |
|-------|--------|-----|
| Backend | Go | Single binary, cross-compiles to ARM, matches NORA |
| Frontend | React + Vite | Fast builds, matches NORA, large ecosystem |
| Database | SQLite | Zero config, embedded, file-based, perfect for single-server |
| Container | Alpine Docker | ~50-60MB image, runs anywhere |
| Config | SQLite + Admin UI | No YAML for end users. All config through the UI. |

### Docker Build

Multi-stage Dockerfile:
1. **Stage 1 (Go):** Build the server binary for target arch (amd64/arm64)
2. **Stage 2 (Node):** Build the Vite frontend (`npm run build`) --- includes both admin UI and display views
3. **Stage 3 (Alpine):** Copy binary + static assets + empty SQLite, expose port

Final image target: under 60MB. Single `docker-compose up` to run.

The only user-facing config is `docker-compose.yml` environment variables for things that must be known before the server starts:

```yaml
services:
  dashboard:
    image: project-name:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data          # SQLite DB persisted here
    environment:
      - PORT=8080             # optional, default 8080
      - DB_PATH=/data/dashboard.db
      # Secrets can be passed as env vars for plugins that reference ${VAR}
      - OWM_API_KEY=your-key
      - HA_TOKEN=your-token
```

Everything else (displays, plugins, themes, layouts) is configured through the admin UI after the container starts.

---

## Folder Structure

```
project-name/
|-- cmd/
|   +-- server/
|       +-- main.go                # Entry point, wires everything together
|
|-- internal/
|   |-- config/
|   |   +-- config.go              # Env var loader, server bootstrap config
|   |
|   |-- shapes/
|   |   |-- registry.go            # Data shape registry + validation
|   |   |-- events.go              # events shape definition
|   |   |-- tasks.go               # tasks shape definition
|   |   |-- weather.go             # weather_current, weather_forecast
|   |   |-- home_devices.go        # home_devices shape
|   |   |-- packages.go            # packages shape
|   |   |-- infrastructure.go      # infrastructure shape
|   |   +-- media.go               # media_status shape
|   |
|   |-- plugins/
|   |   |-- data/
|   |   |   |-- plugin.go          # DataPlugin + DataPluginManifest interfaces
|   |   |   |-- registry.go        # Plugin registration + lifecycle
|   |   |   |-- msgraph/
|   |   |   |   |-- auth.go        # OAuth2 flow (enterprise + consumer)
|   |   |   |   |-- calendar.go    # Calendar data plugin
|   |   |   |   +-- todo.go        # To-Do data plugin
|   |   |   |-- openweathermap/
|   |   |   |   +-- weather.go     # Weather data plugin
|   |   |   |-- homeassistant/
|   |   |   |   +-- devices.go     # Home Assistant data plugin
|   |   |   +-- local/
|   |   |       +-- clock.go       # System clock (no external fetch)
|   |   +-- ui/
|   |       +-- registry.go        # UI plugin registry (server-side metadata)
|   |
|   |-- db/
|   |   |-- sqlite.go              # SQLite connection + migrations
|   |   |-- migrations/            # SQL migration files (numbered)
|   |   |   |-- 001_initial.sql
|   |   |   +-- ...
|   |   |-- store.go               # Generic read/write by data shape
|   |   |-- displays.go            # CRUD for displays, screens, cards
|   |   |-- plugins.go             # CRUD for plugin instances, tokens
|   |   +-- themes.go              # CRUD for themes
|   |
|   |-- scheduler/
|   |   +-- scheduler.go           # Runs data plugins on their intervals
|   |
|   |-- auth/
|   |   |-- auth.go                # Admin auth (JWT, sessions)
|   |   +-- oauth.go               # Generic OAuth2 flow handler
|   |
|   +-- api/
|       |-- router.go              # HTTP router setup (data + admin + display)
|       |-- middleware.go           # CORS, auth middleware, logging
|       |-- data_handlers.go       # /api/data/* endpoints
|       |-- admin_handlers.go      # /api/admin/* endpoints
|       |-- display_handlers.go    # /display/* endpoints
|       +-- oauth_handlers.go      # /api/admin/oauth/* endpoints
|
|-- web/                            # React + Vite frontend
|   |-- src/
|   |   |-- plugins/               # UI plugins, one folder each
|   |   |   |-- calendar-agenda/
|   |   |   |   |-- index.ts       # Plugin registration
|   |   |   |   +-- CalendarAgenda.tsx
|   |   |   |-- task-list/
|   |   |   |-- weather-current/
|   |   |   |-- weather-forecast/
|   |   |   |-- meal-plan/
|   |   |   |-- home-status/
|   |   |   |-- package-tracker/
|   |   |   |-- server-health/
|   |   |   |-- media-now-playing/
|   |   |   +-- clock/
|   |   |
|   |   |-- display/               # Display renderer (what wall screens show)
|   |   |   |-- DisplayApp.tsx     # Top-level display view
|   |   |   |-- Grid.tsx           # Grid engine
|   |   |   |-- GridCell.tsx       # Individual card container
|   |   |   |-- ScreenRotator.tsx  # Screen transition/rotation
|   |   |   |-- TopBar.tsx         # Fixed top bar
|   |   |   +-- BottomBar.tsx      # Fixed bottom bar
|   |   |
|   |   |-- admin/                 # Admin UI
|   |   |   |-- AdminApp.tsx       # Admin shell (sidebar nav, routing)
|   |   |   |-- pages/
|   |   |   |   |-- SetupWizard.tsx    # First-run setup
|   |   |   |   |-- Dashboard.tsx      # Admin home (system overview)
|   |   |   |   |-- Settings.tsx       # System settings
|   |   |   |   |-- Users.tsx          # User management
|   |   |   |   |-- PluginList.tsx     # Browse/manage data plugin instances
|   |   |   |   |-- PluginSetup.tsx    # Add/edit data plugin (manifest-driven wizard)
|   |   |   |   |-- DisplayList.tsx    # Manage displays
|   |   |   |   |-- DisplayDesigner.tsx # Drag-and-drop grid editor
|   |   |   |   +-- ThemeEditor.tsx    # Visual theme token editor
|   |   |   +-- components/
|   |   |       |-- ManifestForm.tsx   # Renders SetupFields from a plugin manifest
|   |   |       |-- OAuthFlow.tsx      # OAuth2 authorization UI
|   |   |       |-- DragDropGrid.tsx   # The drag-and-drop grid surface
|   |   |       |-- CardSettings.tsx   # Per-card config panel
|   |   |       +-- ThemePreview.tsx   # Live theme preview
|   |   |
|   |   |-- shared/                # Shared between display + admin
|   |   |   |-- themes/
|   |   |   |   +-- tokens.ts      # Theme token types + defaults
|   |   |   |-- hooks/
|   |   |   |   |-- usePluginData.ts   # Fetch data for a UI plugin
|   |   |   |   +-- useTheme.ts
|   |   |   +-- types/
|   |   |       |-- plugin.ts      # UIPlugin interface
|   |   |       +-- shapes.ts      # TypeScript types for each data shape
|   |   |
|   |   +-- main.tsx               # Entry point, routes to admin or display
|   |
|   |-- index.html
|   |-- vite.config.ts
|   |-- tsconfig.json
|   +-- package.json
|
|-- Dockerfile                     # Multi-stage: build Go + Vite, ship Alpine
|-- docker-compose.yml
|-- go.mod
|-- go.sum
|-- Makefile
+-- README.md
```

---

## Build Order

### Phase 1: Skeleton + Admin Foundation

- [ ] Go project scaffolding (`cmd/server`, `internal/` structure)
- [ ] Vite + React frontend scaffolding (with admin/display route split)
- [ ] SQLite setup + migration system
- [ ] Data shape registry + validation
- [ ] Plugin interfaces (DataPlugin + DataPluginManifest)
- [ ] Scheduler (runs data plugins on their configured intervals)
- [ ] REST API skeleton (router, middleware, auth)
- [ ] Admin UI: first-run setup wizard (create admin account)
- [ ] Admin UI: login + JWT auth
- [ ] Admin UI: system settings page
- [ ] Docker build (multi-stage)

### Phase 2: Data Plugin Management (prove the pattern)

- [ ] `clock` plugin (no data shape, pure local --- proves the plugin lifecycle)
- [ ] `openweathermap` data plugin + manifest
- [ ] Admin UI: plugin list page
- [ ] Admin UI: manifest-driven setup wizard (add/configure plugin instances)
- [ ] Admin UI: plugin status dashboard (last fetch, errors, row counts)
- [ ] Verify end-to-end: admin adds plugin, wizard renders from manifest, config saves to SQLite, scheduler picks it up, data lands in DB, API serves it

### Phase 3: Display Designer

- [ ] Display/screen/card SQLite CRUD
- [ ] Admin UI: display list (create/edit/delete displays)
- [ ] Admin UI: screen manager (add/reorder/delete screens within a display)
- [ ] Admin UI: drag-and-drop grid editor (place cards, resize, move)
- [ ] Admin UI: card settings panel (UI plugin config from `configSchema`)
- [ ] Admin UI: theme editor (visual token editor with live preview)
- [ ] `weather-current` + `weather-forecast` UI plugins
- [ ] Grid engine (display renderer that reads from SQLite layout)
- [ ] Screen rotation / transitions
- [ ] Display endpoint (`/display/{slug}`)
- [ ] Verify end-to-end: admin designs a display, browser hits the URL, sees the layout

### Phase 4: Microsoft Graph

- [ ] OAuth2 flow handler (generic, reusable across providers)
- [ ] Admin UI: OAuth2 authorization step in plugin setup wizard
- [ ] `msgraph-calendar` data plugin + manifest
- [ ] `calendar-agenda` UI plugin (5-day rolling view, multi-calendar, holidays, birthdays)
- [ ] `msgraph-todo` data plugin + manifest
- [ ] `task-list` UI plugin
- [ ] `meal-plan` UI plugin (calendar-filtered)
- [ ] Test with consumer account (tenant_id: "consumers")
- [ ] Test with enterprise account (Entra ID)

### Phase 5: Home Integration

- [ ] `homeassistant` data plugin + manifest
- [ ] `home-status` UI plugin
- [ ] `server-health` UI plugin (infrastructure shape)
- [ ] `media-now-playing` UI plugin
- [ ] `package-tracker` UI plugin

### Phase 6: Client Support (Server-Side)

Server-side work needed before the client apps can connect. The client apps themselves are separate projects with their own build timelines.

- [ ] `clients` table + migration
- [ ] Client registration API (`POST /api/clients/register`)
- [ ] Client polling API (`GET /api/clients/{client_id}/config`)
- [ ] Offline screen pre-render endpoint (`GET /api/clients/{client_id}/offline`)
- [ ] Admin UI: client management page (list, approve, assign to display, status)
- [ ] Admin UI: per-display offline screen config in Display Designer
- [ ] Pi setup script (`setup-kiosk.sh`) for raw-browser users

### Phase 7: Polish

- [ ] Bundled theme presets (glass dark, solid dark, glass light, etc.)
- [ ] Weather alerts in bottom bar
- [ ] Responsive adjustments for non-16:9 displays
- [ ] Documentation for writing custom data plugins
- [ ] Documentation for writing custom UI plugins
- [ ] Performance tuning for Raspberry Pi (minimize re-renders, optimize bundle size)

---

## Open Questions

- [ ] **Project name** --- needed for Go module path, Docker image, repo name
- [ ] **Google provider abstraction** --- how much do we design for Google parity now vs. later?
- [ ] **Write-back** --- should UI plugins ever write data back (mark a chore done from the wall display)? Input method TBD if so.
- [ ] **SSE vs polling** --- Server-Sent Events would give instant updates for fast-changing data (media, doors). Worth the complexity for v1?
- [ ] **Display authentication** --- dedicated client uses admin-approved registration. Raw browser access is unauthenticated on LAN. Should we support authenticated viewers for remote/off-network access?

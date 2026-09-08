# Architecture

This is the canonical, current-state design reference for Mullet: what's
actually built, in the actual shapes it's built in. It's meant to be kept
up to date as the implementation evolves — when a change here diverges
from what's written, fix this doc in the same PR.

[`docs/architecture_1.md`](docs/architecture_1.md) is the original design
proposal this project started from. It's kept for the detail it still has
on genuinely unbuilt areas (the dedicated client app protocol, the
Microsoft Graph OAuth2 flow), but it has already drifted from what got
built in several places — see [Divergence from the original
proposal](#divergence-from-the-original-proposal) at the bottom. Where
the two disagree, this document wins.

Sections below are marked **Built**, **Planned**, or a split of the two
(e.g. schema built but nothing renders it yet). Planned material
describes intended design for work that hasn't landed, carried over
from the original proposal, and names the issue that will build it.

---

## Overview

A self-hosted, open-source widget dashboard for wall-mounted displays
(Raspberry Pi + monitor, an old tablet, any kiosk browser) — no
subscriptions, no vendor lock-in, no cloud dependency.

**Two-piece system:**
- **Server** (this repo, one Docker image): a Go binary that runs data
  plugins on a schedule, writes to SQLite, serves a REST API, and hosts
  the React frontend (admin UI + display views).
- **Client** (endpoint device): a browser pointed at the server, in
  kiosk mode or otherwise. A dedicated client app is planned (see
  [Client Connection Model](#client-connection-model-planned)) but not
  built yet — today, every display is a plain browser tab.

The server does all the work; the client just renders. One server, many
displays, each with its own URL (`/display/{slug}`).

---

## Plugin System (Built)

Three layers, strict ownership:

```
+------------------------------------------------------+
|                    FRAMEWORK                         |
|   Owns: data shapes, SQLite schema, scheduler,        |
|         REST API, admin UI                            |
+------------+----------------------------+------------+
             |                            |
     +-------v-------+          +--------v-------+
     |  DATA PLUGINS  |          |  UI PLUGINS    |
     |  Fetch from    |  --DB--> |  Read from a   |
     |  an external   |          |  data shape,   |
     |  source, write |          |  render in a   |
     |  to a shape    |          |  grid card     |
     +----------------+          +----------------+
```

The framework owns the data shapes; plugins write to or read from them,
never define their own copy. Two data plugins can write the same shape
(a Microsoft Calendar plugin and the ICS feed plugin both feeding
`events`, say); any UI plugin reading that shape doesn't care which
wrote a given row. (UI plugins — the React side of this — don't exist
yet; see [Divergence](#divergence-from-the-original-proposal).)

Every plugin ships compiled into the Go binary — there is no dynamic
plugin loading at runtime. **Enabling/configuring** a plugin instance is
hot (admin UI → SQLite → scheduler picks it up, no restart). **Adding a
new plugin type** means writing the Go package and rebuilding the image.

### The `DataPlugin` interface

```go
// internal/plugins/data/plugin.go
type DataPlugin interface {
    ID() string
    Name() string
    Manifest() DataPluginManifest
    DataShapes() []string
    RefreshInterval() time.Duration
    Configure(cfg map[string]any) error
    Fetch(ctx context.Context) (map[string][]any, error)
}
```

`Fetch` returns every shape it produces in one call, keyed by shape name
— a plugin that writes both `weather_current` and `weather_forecast`
(openweathermap, open-meteo) returns both in one map. A plugin with no
data of its own (clock) returns an empty map. `Manifest()` describes the
plugin for the admin UI's setup wizard: `SetupFields` renders a form,
and whatever the admin submits is what `Configure` receives.

A plugin registers itself via `init()`:

```go
func init() {
    if err := plugindata.Register(New()); err != nil {
        panic(err)
    }
}
```

`cmd/server/main.go` blank-imports every plugin package for that side
effect — adding a plugin to the binary means implementing the interface
and adding one import line there.

### Registry and concurrency

`plugindata.Registry` (`internal/plugins/data/registry.go`) holds one
shared object per registered plugin *type* — every configured instance
of, say, `openweathermap` shares the same Go object, reconfigured before
each fetch. That sharing means two callers (the scheduler's own tick,
and an admin's manual "test connection") could race on
`Configure()`/`Fetch()` and corrupt each other's config. `Registry.WithPlugin(id, fn)`
serializes access per plugin ID via a per-ID mutex; the scheduler and the
admin "test" endpoint both go through it, never call a plugin directly.

### Scheduler

`internal/scheduler/scheduler.go` runs one goroutine per enabled plugin
instance, ticking on its configured `refresh_seconds`. `Reload()`
reconciles running goroutines against the current DB state — called
after every instance create/update/delete/enable-toggle, so a config
change takes effect without a server restart. `TestInstance(ctx, id)`
runs one configure+fetch cycle synchronously and returns its outcome,
serialized through the same `WithPlugin` path so a manual test can never
interleave with a scheduled tick on the same plugin type.

### Compiled-in plugins

| ID | Shapes written | Auth | Notes |
|---|---|---|---|
| `clock` | (none) | none | Proves the plugin lifecycle; the clock UI plugin (not yet built) will read the browser's own time |
| `open-meteo` | `weather_current`, `weather_forecast` | none | Free, no API key |
| `openweathermap` | `weather_current`, `weather_forecast` | API key | |
| `ics-feed` | `events` | none, or HTTP basic auth | Universal ICS/iCal calendar connector; one plugin instance = one feed/calendar (add the plugin again for a second feed) |

---

## Data Shapes (Built)

`internal/shapes` defines the framework's typed data contracts — plain
Go structs, one per shape (`WeatherCurrent`, `WeatherForecast`,
`CalendarEvent`, `Task`, `HomeDevice`, `Package`, `InfraService`,
`MediaStatus`). A data plugin's `Fetch()` returns these; nothing casts
through `map[string]any` on the write side.

### Write path

`db.WriteShape(sqldb, shape, pluginInstanceID, rows)`
(`internal/db/store.go`) looks up a `shapeWriter` function for the shape
name and runs it in one transaction. Every writer so far follows a
**Replace** strategy: delete this instance's existing rows for the
shape, insert the fresh set. `events` is the one exception —
see below.

**Entity discovery for `events`.** A plugin has no DB handle, so it
can't resolve a real `calendars.id` foreign key itself. Instead
`shapes.CalendarEvent` carries `CalendarExternalID` / `CalendarName` /
`CalendarColor` (write-time-only fields, not real `shape_events`
columns) that a plugin sets on every row it returns. `writeEvents`
groups rows by `CalendarExternalID`, upserts a `calendars` row per
distinct one (the "entity discovery" step — insert on first sight,
update name/color on every fetch after), resolves the real
`calendars.id`, and replaces only that calendar's `shape_events` rows —
scoped by `(plugin_instance_id, calendar_id)`, so a plugin instance
covering multiple calendars in one fetch never clobbers one calendar's
data while updating another's.

### Read path

`GET /api/data/{shape}` (`internal/db/data_api.go`,
`internal/api/data_handlers.go`) reads generically — `map[string]any`
rows scanned straight from the shape table's columns, no typed struct on
this side. Optional query params filter by plugin instance and, for
shapes that support it (`events` today — see `SupportsTimeRange`), a
time range.

---

## SQLite Schema

`internal/db.Open` enables WAL mode and foreign-key enforcement, and
caps the connection pool at 1 (`SetMaxOpenConns(1)`) since SQLite only
allows one writer at a time — simpler than a busy-retry loop.
`internal/db.Migrate` applies the numbered files in
`internal/db/migrations/*.sql` (embedded into the binary via `go:embed`)
in filename order, each in its own transaction, recording applied
filenames in a `schema_migrations` table so a restart never re-applies
one.

### Built

| Table | Purpose |
|---|---|
| `users` | Admin accounts (bcrypt password hash) |
| `system_settings` | Server-level key/value settings, including the persisted JWT signing secret |
| `data_plugin_instances` | Every configured plugin instance: which plugin, its JSON config, enabled flag, refresh interval, last fetch time/error |
| `calendars` | Discovered calendars, one row per `(plugin_instance_id, external_id)` — see [entity discovery](#write-path) |
| `task_lists` | Same shape as `calendars`, for the `tasks` contract (no writer uses this yet) |
| `shape_events`, `shape_tasks`, `shape_weather_current`, `shape_weather_forecast`, `shape_home_devices`, `shape_packages`, `shape_infrastructure`, `shape_media_status` | One table per data shape, typed columns matching its Go struct — no JSON blobs. See `internal/db/migrations/002_shapes.sql`. |
| `themes` | Named JSON token sets (`internal/db/migrations/004_displays.sql`). A display references one as its base theme; a card can override it via `cards.theme_override`. Seeded with one row ("Dark Glass", `is_default`) by `006_seed_default_theme.sql` |
| `displays` | A physical output routed at `/display/{slug}` (unique). `theme_id` nullable FK to `themes`; `rotation_seconds` how often it rotates through its screens; `show_top_bar`/`show_bottom_bar` toggle the fixed clock/weather and now-playing/alerts bars |
| `screens` | A page within a display (`ON DELETE CASCADE` from `displays`). Owns its own grid (`columns`, `row_height`, `gap`, all in pixels except `columns`) rather than inheriting one from its display; `position` orders rotation |
| `cards` | A positioned UI plugin on a screen's grid (`ON DELETE CASCADE` from `screens`). `x`/`y`/`w`/`h` are grid units. `data_plugin_instance_id` (nullable, `ON DELETE SET NULL`) is which configured plugin instance it reads from — nullable because a card's UI plugin might need no data (clock) or the admin hasn't wired one up yet; `SET NULL` rather than cascade so deleting an unrelated data plugin instance doesn't silently delete a card |

Every shape table carries `plugin_instance_id` (`ON DELETE CASCADE` from
`data_plugin_instances`) and `fetched_at`. `shape_events`/`shape_tasks`
additionally cascade from `calendars`/`task_lists`.

CRUD for all four tables above is built (see [REST API Routes](#rest-api-routes)
below), and so is an admin UI to manage all of it, including a
drag-and-drop Designer for cards; see
[Display Hierarchy](#display-hierarchy-designer-built-display-side-rendering-planned)
and [Theme Cascade](#theme-cascade-editor-built-display-side-application-planned).
The one thing that still doesn't render any of this is the display
frontend itself (`/display/{slug}`).

### Planned

Full column lists for what's not built are in
[`docs/architecture_1.md` § SQLite Schema](docs/architecture_1.md#sqlite-schema).

| Table | Purpose | Lands with |
|---|---|---|
| `oauth_tokens` | Access/refresh tokens for OAuth2 data plugins (Microsoft Graph, etc.) | Whichever OAuth2 plugin needs it first |
| `clients` | Registered dedicated-client-app devices | #29 |

---

## REST API Routes

### Built

All routes below are wired in `internal/api/router.go`.

| Method & Path | Auth | Purpose |
|---|---|---|
| `GET /healthz` | none | Liveness check |
| `POST /api/admin/setup` | none (first-run only) | Create the initial admin account |
| `GET /api/admin/setup` | none | Whether setup has already run |
| `POST /api/admin/login` | none | Exchange username/password for a JWT |
| `GET /api/admin/me` | JWT | Whoami, exercises the auth guard |
| `GET /api/admin/dashboard` | JWT | Server info + at-a-glance plugin status |
| `GET`/`PUT /api/admin/settings` | JWT | System settings |
| `GET /api/admin/plugins` | JWT | List registered plugin types + manifests |
| `GET`/`POST /api/admin/plugins/instances` | JWT | List / create plugin instances |
| `PUT`/`DELETE /api/admin/plugins/instances/{id}` | JWT | Update / remove an instance |
| `POST /api/admin/plugins/instances/{id}/test` | JWT | Run one configure+fetch cycle now, report success/error |
| `GET`/`POST /api/admin/themes` | JWT | List / create themes |
| `GET`/`PUT`/`DELETE /api/admin/themes/{id}` | JWT | Get / update / remove a theme (`DELETE` is `409` if a display still uses it) |
| `GET`/`POST /api/admin/displays` | JWT | List / create displays |
| `PUT`/`DELETE /api/admin/displays/{id}` | JWT | Update / remove a display (delete cascades to its screens and cards) |
| `GET`/`POST /api/admin/displays/{id}/screens` | JWT | List / create screens on a display |
| `GET`/`PUT`/`DELETE /api/admin/screens/{id}` | JWT | Get / update / remove a screen (delete cascades to its cards) |
| `GET`/`POST /api/admin/screens/{id}/cards` | JWT | List / create cards on a screen |
| `PUT`/`DELETE /api/admin/cards/{id}` | JWT | Update / remove a card |
| `GET /api/data/{shape}` | none (LAN-facing, like the display itself) | Typed rows for a data shape, optionally filtered |

`requireAuth` (`internal/api/middleware.go`) guards every `/api/admin/*`
route except setup/login behind a `Bearer <jwt>` header — unless
`AUTH_DISABLED=true`, a local-dev-only bypass that skips the check
entirely (never set in a real deployment; the server logs a warning on
startup if it's on).

Everything else — `/admin`, `/display/{slug}`, and their static assets —
falls through to the built frontend's `index.html` (client-side routed
via react-router). `/display/{slug}` currently just renders an
unstyled placeholder (`DisplayApp.tsx`); there's no display-config API
for it to read yet.

### Planned

`/api/clients/*` is mounted (`router.go`) but has no handlers — it's a
placeholder for #29. Routes for displays/screens/cards/themes and a
richer client API are proposed in
[`docs/architecture_1.md` § API Endpoints](docs/architecture_1.md#api-endpoints);
expect the exact paths to differ from that proposal the way the built
routes above already do (see [Divergence](#divergence-from-the-original-proposal)).

---

## Display Hierarchy (Designer built; display-side rendering planned)

The `displays`/`screens`/`cards` tables and their full admin CRUD API
exist (see [SQLite Schema](#sqlite-schema) and [REST API Routes](#rest-api-routes)
above), and so does an admin UI for the whole hierarchy: `/admin/displays`
(`DisplaysPage.tsx`) lists, creates, edits, and deletes displays (name,
slug, theme, rotation interval, top/bottom bar toggles) and lets an
admin manage a display's screens inline -- add, edit, delete, and
reorder (simple ▲/▼ buttons swapping `position`, not drag-and-drop).
Each screen's "Design" button opens `DesignerPage.tsx` -- a drag-and-drop
grid editor (built on `react-grid-layout`) for placing, moving, resizing,
and configuring cards, described below.

What's still planned: the display frontend actually *reading* any of
this to render a live grid -- `/display/{slug}` is still just a
placeholder (`DisplayApp.tsx`) with no data fetch of its own -- and real
UI plugin components for a card to render (see [Designer palette](#the-designer)
below). Lands with #23.

```
Display   "Kitchen"  --  /display/kitchen
  +-- Screen 1  "Main"    -- rotation_seconds on the *display* controls
  |     |                    how long each screen stays up
  |     +-- Card  x:1 y:1 w:4  h:10 --> calendar-agenda (UI plugin)
  |     +-- Card  x:5 y:1 w:8  h:3  --> weather-forecast (UI plugin)
  +-- Screen 2  "Detail"
        +-- Card  x:1 y:1 w:16 h:12 --> full-screen calendar view
```

- **Display** (`displays`): a named physical output, unique `slug` for
  `/display/{slug}` routing, an optional base `theme_id`, and
  `rotation_seconds` — how long the display stays on each screen before
  advancing to the next.
- **Screen** (`screens`): a page within a display (`ON DELETE CASCADE`).
  Owns its *own* grid — `columns`, `row_height`, and `gap` (pixels) —
  rather than inheriting fixed dimensions from its display, so one
  screen can be a dense multi-card layout and another a single
  full-bleed card. `position` orders the rotation.
- **Card** (`cards`): a positioned rectangle on its screen's grid
  (`x`/`y`/`w`/`h`, grid units, `ON DELETE CASCADE` from the screen),
  referencing a UI plugin (`ui_plugin_id`, a free-form string — there's
  no server-side UI plugin registry to validate it against) plus its own
  JSON `config`, an optional data source (`data_plugin_instance_id`,
  nullable, `ON DELETE SET NULL`), and an optional `theme_override`.
- **UI plugin**: the React component rendered inside a card, reading one
  data shape. Two exist (`weather-current`, `weather-forecast` — see
  [UI Plugins](#ui-plugins-two-built-no-consumer-yet) below); the rest
  implied by the plugins above are still to be built.

### The Designer

`DesignerPage.tsx` (`/admin/displays/{displayId}/screens/{screenId}/design`)
renders one screen's cards on a `react-grid-layout` grid sized to that
screen's `columns`/`row_height`/`gap`. Drag a palette entry onto the
grid to create a card (`POST .../cards`); drag or resize an existing one
(all 8 handles) to reposition it (`PUT .../cards/{id}`, fired on
drag/resize *stop*, not on every intermediate frame); the grid's default
compactor keeps cards from ever overlapping, reflowing instead of
allowing a collision. A card's gear icon opens a settings panel: data
source (a dropdown of configured plugin instances, filtered to ones
whose plugin manifest lists the shape this card type reads), a raw JSON
config editor, and an optional theme override -- a toggle plus the
narrow `ThemeTokenFields` set described in
[Theme Cascade](#theme-cascade-editor-built-display-side-application-planned)
(#21), not raw JSON.

The palette itself is **not** driven by the UI plugin registry described
below — it's a small hardcoded list in `DesignerPage.tsx` scoped to
shapes a compiled-in data plugin can actually produce today (`clock`,
`weather-current`, `weather-forecast`, `calendar-agenda`). A placed card
still renders as a generic labeled box (widget name + data source), not
real widget content, even for the two UI plugins that now exist --
wiring the Designer (or a live display) up to actually render them is
issue #23's job, not this one's.

### UI Plugins (Two built, no consumer yet)

`web/src/plugins/` holds real UI plugin implementations, each in its own
folder per `UIPlugin` (`web/src/shared/types/plugin.ts`): an `id`,
`dataShape`, `defaultSize`/`minSize`/`maxSize` (grid units), an optional
`configSchema`, and a `component` receiving `WidgetProps<TData>` --
`data` (the shape's rows, exactly as `GET /api/data/{shape}` returns
them -- nothing fetches on the widget's behalf), `config`, `size`
(current grid units, the one hint a widget gets about its own room --
there's no ResizeObserver or pixel measurement), and `theme`. Two exist:

- **`weather-current`**: temp, condition glyph, high/low, humidity, wind
  speed. Drops the secondary stats when `size` is small (`≤2` grid
  rows or `≤3` columns) rather than overflowing.
- **`weather-forecast`**: one column per forecast day (glyph, high/low,
  precip chance), the day count itself capped by both a `days` config
  option and by `size.w` (a narrow card shows 2 days, not all 5,
  squeezed into columns).

Both render their **own** full card chrome from `theme` (background,
border, radius, blur, opacity, font) -- there's no separate wrapping
`Card` component in this design, matching how the Designer's own
placeholder cards already work. Both key their icon off the shape row's
`condition` string (`"Clear"`, `"Rain"`, ...), not its `icon` field: the
two weather data plugins don't share an icon vocabulary (openweathermap
passes through OWM's own codes like `"02d"`; open-meteo emits plain
keywords like `"clouds"`) but both normalize `condition` to the same
small set of category words, so that's the one field a UI plugin can
actually trust regardless of which data plugin produced the row (see
`web/src/plugins/shared/conditionIcons.ts`).

`web/src/plugins/registry.ts` lists every built UI plugin (mirrors
`internal/plugins/data/registry.go`'s pattern on the Go side), for
whatever eventually looks up "which component renders `ui_plugin_id`
X" -- nothing does yet. Until #23 builds that consumer, these two
components are verified only by direct testing against the real API,
not by anything in the running app.

`shapes.WeatherCurrent` gained a `wind_speed` column (migration
`007_weather_wind_speed.sql`) to back the wind reading -- it didn't
exist before `weather-current` needed to display it. Both weather data
plugins (openweathermap, open-meteo) were updated to populate it.

---

## Theme Cascade (Editor built; display-side application planned)

The `themes` table and its admin CRUD API exist (a theme is just a name
plus a `tokens` JSON blob and an `is_default` flag — see
[SQLite Schema](#sqlite-schema)/[REST API Routes](#rest-api-routes)),
and so does a full editor: `/admin/themes` (`ThemesPage.tsx`) lists
themes with a swatch preview and lets an admin create/edit/delete them;
`/admin/themes/{id}` (`/new` for a new one) is `ThemeEditorPage.tsx` --
a field editor for every token (background type/value, card background/
border, text/accent color, font family/size, border radius, opacity,
blur) alongside a live preview panel that re-renders a small sample
card grid on every keystroke, using the same styles the eventual display
renderer will need. `/admin/displays` (#19) lets an admin *pick* a theme
for a display from a dropdown, defaulting to whichever theme has
`is_default` set when adding a new display. The Designer's card settings
panel (#20) now edits `theme_override` through the same field-editor
component, restricted to a narrow field set (see below) instead of the
raw JSON box it started with.

What's *still* not built: anything on the display side that actually
*reads* a display's theme and its cards' overrides to render a live
grid -- that's #23. The Designer's own card placeholders and the theme
editor's preview both hand-roll their own styling from the token values
directly; neither goes through a shared "apply a theme" renderer, because
that renderer doesn't exist yet.

The token **type** all of this reads and writes is a single shared
source: [`web/src/shared/themes/tokens.ts`](web/src/shared/themes/tokens.ts)
defines `ThemeTokens` and a built-in `defaultTheme`, consumed via a
`ThemeContext` (`useTheme.ts`) on the display side (still unpopulated
from the database) and via `ThemeTokenFields.tsx` (a component shared
between the full theme editor and the card override editor) on the
admin side. The bundled default theme ("Dark Glass", seeded by
migration `006_seed_default_theme.sql`) uses these exact token values,
so there's never a fresh install with zero themes to pick from. This
same token shape also backs a separate, static admin-UI theme system
(`web/src/admin/adminTheme.css`) that is *not* part of this cascade --
that one styles the admin app itself and isn't DB-configurable.

Cascade, once something renders it: a **Display** sets a base theme (FK
to `themes`); its **Screens'** cards inherit it by default; a **Card**
can override specific tokens via `theme_override` (nullable — null
means full inheritance, which is what `CreateCard`/`UpdateCard` store
when it's omitted). Overrides are deliberately narrow — the card
settings panel only exposes `cardBackground`, `accentColor`, and
`opacity` — so one card can't take over a display's whole look. That
narrowing lives only in the frontend (`CARD_OVERRIDE_FIELDS` in
`DesignerPage.tsx`); the API itself accepts a `theme_override` with any
subset of token keys, same as it accepts any JSON object for a theme's
full `tokens`.

---

## Client Connection Model (Planned)

*Lands with #29. Full detail, including the registration pairing flow,
offline-mode design, and per-platform kiosk setup, is in
[`docs/architecture_1.md` § Client`](docs/architecture_1.md#client) —
summarized here.*

Today, a display is just `GET /display/{slug}` in a plain browser: no
registration, no offline fallback beyond whatever the browser itself
shows when the server is unreachable.

The planned design adds a second, recommended path — a lightweight
dedicated client app (separate repo; Go+webview on Raspberry Pi/Linux,
Kotlin on Android/Android TV) that:
- registers with the server via a short-lived pairing code, approved
  once in the admin UI;
- polls for its display assignment (default 15s) rather than holding a
  persistent connection;
- on server unreachable, falls back to a pre-rendered, server-supplied
  offline screen cached locally (not a frozen copy of the live
  dashboard) — configurable to instead power off the screen via
  HDMI-CEC (Raspberry Pi only) or freeze on the last frame;
- on Raspberry Pi with HDMI-CEC, can also drive a screen-on/off schedule
  independent of connectivity.

The raw-browser path (what exists today) stays supported indefinitely
for devices where installing a client isn't practical — it just won't
get registration, offline fallback, or screen power management. The
React display app is expected to grow its own lightweight resilience
(a "reconnecting" overlay over the last good frame on a failed poll)
independent of whether the dedicated client ships.

---

## Auth Model (Built)

- Passwords: bcrypt (`internal/auth/password.go`, `bcrypt.DefaultCost`).
- Sessions: JWT, HS256, 24h TTL (`internal/auth/jwt.go`). The signing
  secret is generated once (32 random bytes) and persisted in
  `system_settings`, so tokens survive a server restart without any
  manual secret management (`internal/auth/secret.go`).
- First run: `GET /api/admin/setup` reports whether an account already
  exists; `POST /api/admin/setup` creates the first admin and returns a
  token, the same shape as login.
- `AUTH_DISABLED=true` (env var, `internal/config`) skips all of the
  above — every `/api/admin/*` request is treated as an authenticated
  dev user. Local development only; the server logs a startup warning
  when it's set, and it must never be set in a real deployment.

---

## Tech Stack

- **Server**: Go 1.26. `modernc.org/sqlite` (pure Go, no cgo) rather
  than `mattn/go-sqlite3` — chosen specifically so the server
  cross-compiles for ARM (Raspberry Pi) without a C toolchain.
  `golang-jwt/v5`, `golang.org/x/crypto/bcrypt` for auth.
  `arran4/golang-ical` + `teambition/rrule-go` for the ICS feed plugin's
  calendar parsing and RRULE recurrence expansion.
- **Frontend**: React 19, TypeScript, Vite, `react-router-dom` v7. No
  general UI component library — hand-rolled CSS. The one exception is
  `react-grid-layout`, used specifically for the Designer's drag/resize/
  collision-detection grid mechanics (`DesignerPage.tsx`) — reimplementing
  that correctly by hand wasn't worth the risk for what a mature,
  purpose-built library already does well.
- **Storage**: SQLite, one file, schema-migrated on startup
  (`internal/db/migrations`, applied in order, tracked so each runs
  once).

---

## Docker / Build

Multi-stage `Dockerfile`: a `golang:1.26-alpine` builder
(`CGO_ENABLED=0`, cross-compiled for `$TARGETOS`/`$TARGETARCH`), a
`node:24-alpine` builder for the Vite frontend, and an `alpine:3.21`
runtime that just copies the server binary and the built frontend —
a 31MB final image. The runtime stage installs `ca-certificates` but not
`tzdata`; the server binary instead blank-imports `time/tzdata`
(`cmd/server/main.go`) to embed the IANA time zone database directly, so
`TZID`-qualified calendar times (the ICS feed plugin's VTIMEZONE
handling) resolve correctly even without the OS's own zoneinfo files.

`DB_PATH` (default `/data/mullet.db`, a mounted volume in the Docker
image) and `PORT` (default `8080`) are the two settings that must come
from the environment, since the server needs them before it can read
anything from its own database.

---

## Folder Structure

```
cmd/server/            Entry point: config, migrations, plugin registration, router, serve
internal/
  api/                 HTTP handlers, router, auth middleware, SPA fallback
  auth/                JWT issuing/parsing, password hashing, JWT secret persistence
  config/               Environment-variable config loading
  db/                   SQLite open/migrate, typed shape writers, generic shape reader,
                         plugin instance CRUD, settings
  db/migrations/        Numbered .sql files, applied once each on startup
  plugins/data/          DataPlugin interface + registry
    clock/  openmeteo/  openweathermap/  icsfeed/     One package per compiled-in plugin
  scheduler/             Per-instance fetch loop, hot reload, manual test-fetch
  shapes/                Framework-owned data contract structs
web/
  src/
    admin/               Admin SPA: pages, layout, auth context, its own static theme (adminTheme.css)
    display/             Display SPA (currently a placeholder — see Display Hierarchy)
    shared/               Types and hooks shared by both (theme tokens, plugin data hooks)
```

---

## Divergence from the original proposal

Concrete places the implementation departed from
[`docs/architecture_1.md`](docs/architecture_1.md)'s original design —
noted here so that doc's specifics aren't taken as current fact.

- **`DataPlugin` interface.** The proposal had `Fetch(ctx) ([]any, error)`
  (one shape per plugin) plus separate `Contract()` and
  `DiscoverEntities(ctx) error` methods. The built interface has
  `Fetch(ctx) (map[string][]any, error)` (a plugin can write several
  shapes in one cycle — openweathermap writes both
  `weather_current` and `weather_forecast`) and `DataShapes() []string`
  instead of `Contract()`. There is no `DiscoverEntities` step; for the
  one shape that needs entity discovery (`events`), the plugin just sets
  `CalendarExternalID`/`Name`/`Color` on each row it returns, and the
  framework's `writeEvents` writer does the upsert — see
  [Write path](#write-path).
- **Route paths** differ throughout: `/api/admin/login` and
  `/api/admin/setup` rather than nested under `/api/admin/auth/*`;
  `/api/admin/plugins` and `/api/admin/plugins/instances` rather than
  nested under `/api/admin/plugins/data/*`; `/api/data/{shape}` rather
  than `/api/data/shapes/{shape}`.
- **`system_settings`**, not `settings`, is the key/value settings
  table name.
- **Plugin manifest interval fields** round-trip through the API as
  whole seconds (`recommended_interval_seconds`, `min_interval_seconds`)
  rather than the Go-side `time.Duration` the proposal's JSON sketch
  implied.
- **Display/screen/card schema** (#18) departs from the proposal in three
  ways: the grid (`columns`/`row_height`/`gap`) lives on `screens`, not
  fixed `grid_cols`/`grid_rows` on `displays` -- each screen can have its
  own layout. Card position is `x`/`y`/`w`/`h`, not `col`/`row`/
  `width`/`height`. And `cards` gained a `data_plugin_instance_id` FK the
  proposal didn't have -- a card needs to know *which* configured plugin
  instance to read from (there can be more than one of the same plugin
  type, e.g. two `ics-feed` instances for two calendars), not just which
  UI plugin renders it.
- **`displays.show_top_bar`/`show_bottom_bar`** (#19) aren't in the
  proposal's `displays` table at all -- the fixed top/bottom bar zones
  were described only in the Grid System diagram, with no way to turn
  them off per display. Added as plain booleans (default on) rather
  than folding them into `theme_id`'s JSON, since they're structural
  (whether a zone renders) rather than a visual token.
- **The Designer's palette** (#20) isn't backed by a UI plugin registry
  the proposal implied would exist by this point -- there still isn't
  one (no UI plugins are built yet at all). It's a hardcoded list in
  `DesignerPage.tsx` scoped to shapes a compiled-in data plugin can
  actually produce, and a placed card renders as a generic labeled
  placeholder, not real widget content. See [The Designer](#the-designer).
- **The theme editor's live preview** (#21) is a standalone page with
  its own generic 3-card mockup, not the proposal's description of a
  preview "on the grid editor" (i.e. live-editing a theme while looking
  at an actual screen's real cards). The GitHub issue that actually
  scoped this work asked for "a live preview panel with sample card
  grid," which is what got built -- simpler, and not dependent on the
  Designer or a real screen existing yet.

When you find another one of these while implementing an issue, add it
here rather than silently leaving the proposal doc wrong.

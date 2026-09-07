# Mullet Data Sources

> Reference for all data sources the framework supports or plans to support.
> Each data source is implemented as a **data plugin** that writes to a typed **data contract**.

---

## V1 Data Plugins (Ship With First Release)

### Microsoft Calendar (`msgraph-calendar`)

| Field | Value |
|-------|-------|
| **Contract** | `events` |
| **Source** | Microsoft Graph API |
| **Auth** | OAuth2 (consumer + Entra ID) |
| **Min Interval** | 60s |
| **Recommended Interval** | 5 min |

Syncs calendars from Outlook / Microsoft 365. Discovers available calendars on connect, lets the user toggle which ones to sync. Writes to `shape_events` with `calendar_id` foreign key. Supports both personal Microsoft accounts (`tenant_id: consumers`) and enterprise Entra ID tenants.

**Scopes:** `Calendars.Read`, `Calendars.ReadBasic`, `offline_access`

**Metadata table:** `calendars` (name, color, enabled per calendar)

---

### Microsoft To Do (`msgraph-todo`)

| Field | Value |
|-------|-------|
| **Contract** | `tasks` |
| **Source** | Microsoft Graph API |
| **Auth** | OAuth2 (consumer + Entra ID) |
| **Min Interval** | 60s |
| **Recommended Interval** | 5 min |

Syncs task lists from Microsoft To Do. Discovers lists on connect, user toggles which to sync. Writes to `shape_tasks` with `task_list_id` foreign key. Shares the OAuth2 token with `msgraph-calendar` when both are configured under the same Microsoft account.

**Scopes:** `Tasks.Read`, `offline_access`

**Metadata table:** `task_lists` (name, enabled per list)

---

### ICS Calendar Feed (`ics-feed`)

| Field | Value |
|-------|-------|
| **Contract** | `events` |
| **Source** | Any ICS/iCal URL |
| **Auth** | None (public URL) or basic auth |
| **Min Interval** | 60s |
| **Recommended Interval** | 15 min |

Universal calendar connector. Fetches and parses `.ics` feeds from any source: Google Calendar (public share link), Apple iCloud (public calendar URL), Nextcloud, Fastmail, school/sports league calendars, municipal event feeds. No OAuth required.

Handles `RRULE`, `EXDATE`, `VTIMEZONE`, and recurring event expansion within a configurable window (default: 60 days forward). Multiple ICS URLs can be added as separate plugin instances, all writing to the same `events` contract.

**Setup fields:** URL, display name, color, optional basic auth credentials

**Metadata table:** `calendars` (one entry per feed URL)

---

### OpenWeatherMap (`openweathermap`)

| Field | Value |
|-------|-------|
| **Contract** | `weather_current`, `weather_forecast` |
| **Source** | OpenWeatherMap API |
| **Auth** | API key |
| **Min Interval** | 300s (free tier: 1000 calls/day) |
| **Recommended Interval** | 15 min |

Current conditions and 5-day forecast. Writes to both `shape_weather_current` (single row, replaced each fetch) and `shape_weather_forecast` (one row per day, replaced each fetch).

**Setup fields:** API key, location (city name, zip, or lat/lon), units (imperial/metric)

**Metadata table:** None (standalone)

---

### Clock (`clock`)

| Field | Value |
|-------|-------|
| **Contract** | None (local) |
| **Source** | System clock |
| **Auth** | None |
| **Interval** | N/A (rendered client-side) |

No external data fetch. The clock UI plugin reads the browser's local time directly. This plugin exists to prove the plugin lifecycle with zero external dependencies.

**Setup fields:** Timezone override (optional), 12/24 hour format

---

## V1 Data Contracts

These are the typed Go structs in the `shapes` package that data plugins write to and UI plugins read from.

### `events` (CalendarEvent)

Written by: `msgraph-calendar`, `ics-feed`
Read by: `calendar-agenda`, `meal-plan`

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | External event ID |
| `plugin_instance_id` | INTEGER | FK to `data_plugin_instances` |
| `calendar_id` | INTEGER | FK to `calendars` |
| `title` | TEXT | Event title |
| `start` | DATETIME | Start time |
| `end` | DATETIME | End time (nullable for open-ended all-day) |
| `all_day` | INTEGER | Boolean |
| `location` | TEXT | Nullable |
| `description` | TEXT | Nullable |

**Write strategy:** Upsert (scoped by `plugin_instance_id` + `calendar_id`)
**Retention:** Window, 60 days

---

### `tasks` (Task)

Written by: `msgraph-todo`
Read by: `task-list`

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | External task ID |
| `plugin_instance_id` | INTEGER | FK to `data_plugin_instances` |
| `task_list_id` | INTEGER | FK to `task_lists` |
| `title` | TEXT | Task title |
| `completed` | INTEGER | Boolean |
| `due_date` | TEXT | Date string, nullable |
| `sort_order` | INTEGER | Display order |

**Write strategy:** Upsert (scoped by `plugin_instance_id` + `task_list_id`)
**Retention:** Latest

---

### `weather_current` (WeatherCurrent)

Written by: `openweathermap`
Read by: `weather-current`

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | Typically "current" |
| `plugin_instance_id` | INTEGER | FK to `data_plugin_instances` |
| `temp` | REAL | Temperature |
| `feels_like` | REAL | Nullable |
| `condition` | TEXT | "Partly Cloudy" |
| `icon` | TEXT | Icon key or emoji |
| `humidity` | INTEGER | Percentage, nullable |
| `high` | REAL | Today's high, nullable |
| `low` | REAL | Today's low, nullable |
| `sunrise` | TEXT | Nullable |
| `sunset` | TEXT | Nullable |

**Write strategy:** Replace
**Retention:** Latest

---

### `weather_forecast` (WeatherForecast)

Written by: `openweathermap`
Read by: `weather-forecast`

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT | Typically the date string |
| `plugin_instance_id` | INTEGER | FK to `data_plugin_instances` |
| `date` | TEXT | Forecast date |
| `high` | REAL | Day high |
| `low` | REAL | Day low |
| `condition` | TEXT | Forecast condition |
| `icon` | TEXT | Icon key |
| `precip_chance` | INTEGER | 0-100, nullable |

**Write strategy:** Replace
**Retention:** Latest

---

## Future Data Plugins (Post-V1)

These use either existing framework contracts for interoperability or define custom contracts for specialized data.

| Plugin | Contract | Source | Auth | Notes |
|--------|----------|--------|------|-------|
| Google Calendar | `events` | Google Calendar API | OAuth2 | Drop-in alongside MS Calendar, same UI plugins |
| Google Tasks | `tasks` | Google Tasks API | OAuth2 | Same deal |
| Home Assistant | `home_devices` (custom) | HA REST API | Long-lived token | Door/garage/sensor states |
| Portainer | `infrastructure` (custom) | Portainer API | API key | Container status |
| PRTG | `infrastructure` (custom) | PRTG API | API key | Network monitor sensors |
| Plex / Jellyfin | `media_status` (custom) | Media server API | API key | Now playing |
| Package tracking | `packages` (custom) | 17track / carrier APIs | API key | Delivery ETAs |
| Todoist | `tasks` | Todoist API | API key | Alternative task source |
| Trello | `tasks` | Trello API | API key | Board cards as tasks |
| RSS/Atom | `articles` (custom) | Any feed URL | None | News/blog headlines |
| Spotify | `media_status` (custom) | Spotify API | OAuth2 | Currently playing |

### Interoperability Principle

Any plugin writing to a **framework contract** (`events`, `tasks`, `weather_current`, `weather_forecast`) is interchangeable. Google Calendar and Microsoft Calendar both write `events`. Any events UI plugin reads from either without knowing or caring which source wrote the data.

Plugins writing to a **custom contract** are on their own island. A `home_devices` UI plugin only reads from data plugins that write `home_devices`. No cross-pollination with framework contracts.

---

## Auth Patterns

| Pattern | Plugins | How It Works |
|---------|---------|--------------|
| **None** | `clock`, `ics-feed` (public URLs) | No credentials needed |
| **API Key** | `openweathermap`, future infra plugins | Single key entered in setup wizard, stored in plugin config JSON |
| **Basic Auth** | `ics-feed` (private URLs) | Username + password entered in setup, stored in plugin config |
| **OAuth2** | `msgraph-calendar`, `msgraph-todo`, future Google plugins | Full authorization code flow with PKCE, tokens in `oauth_tokens` table, auto-refresh before expiry |
| **Long-lived Token** | future `homeassistant` | Token generated in the source app, pasted into setup wizard |

---

## Plugin Manifest (What Drives the Admin UI)

Every data plugin declares a manifest that the admin UI reads to auto-generate its setup wizard. No hardcoded forms per plugin.

```
DataPluginManifest
  ID                  -- "msgraph-calendar"
  Name                -- "Microsoft Calendar"
  Description         -- "Syncs calendars from Outlook / Microsoft 365"
  Contract            -- "events"
  SetupFields[]       -- drives the wizard form (text, select, toggle, password, etc.)
  AuthType            -- "none" | "api_key" | "oauth2"
  OAuthConfig         -- auth/token URLs, scopes (only if AuthType == "oauth2")
  RecommendedInterval -- suggested polling frequency
  MinInterval         -- hard floor (API rate limit guardrail)
```

The admin UI renders the setup form entirely from `SetupFields`. Adding a new data plugin to the system means implementing the Go interface and declaring the manifest. Zero frontend work for plugin-specific setup screens.

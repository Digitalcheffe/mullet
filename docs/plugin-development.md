# Plugin Development Guide

Mullet has two kinds of plugin, both compiled into their respective
binaries -- there's no dynamic loading, no plugin marketplace, no
`.so`/`.wasm` files dropped into a folder. Adding a plugin means writing
a Go package or a React component, in this repo, and rebuilding.

- A **data plugin** (Go, server-side) fetches from some external source
  on a schedule and writes typed rows to a **data shape** (a SQLite
  table with a fixed, framework-owned column set).
- A **UI plugin** (React, client-side) reads one data shape and renders
  it as a card on a display's grid.

Neither talks to the other directly -- they only share the shape's name
and column set. Two data plugins can write the same shape (a Microsoft
Calendar plugin and an ICS feed plugin both feeding `events`, say) and
any UI plugin reading that shape doesn't care which one produced a given
row. See [`architecture.md`](../architecture.md)'s "Plugin System" and
"Data Shapes" sections for the authoritative reference this guide walks
through in practice -- if the two ever disagree, architecture.md wins.

Two complete, working reference templates back this guide:
[`internal/plugins/data/exampleplugin/`](../internal/plugins/data/exampleplugin/)
and
[`web/src/plugins/example-widget/`](../web/src/plugins/example-widget/).
Both compile, both have tests/type-check cleanly, and both are
deliberately *not* wired into the running app (not blank-imported by
`cmd/server/main.go`, not listed in `registry.ts`) -- read them
alongside this guide, then copy the directory as your actual starting
point.

---

## Writing a data plugin

A data plugin implements `plugindata.DataPlugin`
(`internal/plugins/data/plugin.go`):

```go
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

### 1. Pick a shape (or add one -- see below)

Decide which existing shape(s) your plugin will produce. If you need a
new one, do that first (next section), then come back here.

### 2. Scaffold the package

```
internal/plugins/data/yourplugin/
  plugin.go
  plugin_test.go
```

`ID()` is a short, unique, stable string (`"openweathermap"`,
`"home-assistant"`) -- it's persisted in
`data_plugin_instances.plugin_id` once anyone configures an instance, so
renaming it later orphans every existing instance. Don't reuse another
plugin's ID.

### 3. Write the manifest

`Manifest()` returns a `DataPluginManifest` describing your plugin to
the admin UI's setup wizard:

- `SetupFields` becomes the actual form (`Type`: `"text"`, `"password"`,
  `"select"`, `"multi-select"`, `"number"`, `"toggle"`; see
  `plugindata.SetupField`'s doc comment for `Required`/`Default`/
  `Placeholder`/`HelpText`/`Options`).
- `AuthType` is `"none"`, `"api_key"`, or `"oauth2"`. For `"oauth2"`, also
  set `OAuthConfig` (`AuthURL`, `TokenURL`, `Scopes`, `TenantField`) --
  see [OAuth2](../architecture.md#oauth2-built) and
  `internal/plugins/data/msgraphcalendar/` for a real example; the
  generic OAuth2 flow handler (`internal/oauth`) does the actual token
  exchange/refresh for you, your plugin just reads
  `cfg["access_token"]` in `Configure`/`Fetch` like any other config
  value.
- `RecommendedInterval`/`MinInterval` bound what the admin can set as
  the instance's poll interval -- pick something that respects the
  upstream API's own rate limits.

### 4. Implement Configure and Fetch

`Configure(cfg map[string]any)` runs before *every* `Fetch` call (not
just once) -- the scheduler shares one plugin object across every
configured instance of your plugin type, reconfiguring it right before
each tick (see `Registry.WithPlugin` in
[Registry and concurrency](../architecture.md#registry-and-concurrency)
for why). Type-assert each field out of `cfg`, validate, store on the
struct, return an error for anything missing/invalid.

`Fetch(ctx) (map[string][]any, error)` does the actual work and returns
every shape you produce in one call, keyed by shape name -- a plugin
producing two shapes (like `openweathermap`, which returns both
`weather_current` and `weather_forecast`) returns both from the same
`Fetch`. Each row must be the shape's real Go struct value (e.g.
`shapes.WeatherCurrent{...}`), not a map -- the write path type-asserts
it (see "Adding a new data shape" below).

A failed `Fetch` fails the whole cycle -- there's no partial write. If
you can get *some* shapes but not others, decide deliberately whether
that's an error (openweathermap's current+forecast fail together) or
something to degrade gracefully around (home-assistant's optional area
lookup returns `nil`, not an error, on failure -- see
`internal/plugins/data/homeassistant/homeassistant.go`'s `fetchAreas`).

### 5. Register it

```go
func init() {
    if err := plugindata.Register(New()); err != nil {
        panic(err)
    }
}
```

Then blank-import the package from `cmd/server/main.go`'s existing list
of data plugin imports:

```go
_ "github.com/Digitalcheffe/mullet/internal/plugins/data/yourplugin"
```

That one line is what actually makes it show up in the admin UI's
Available Plugins list -- nothing else does. This is the step
`exampleplugin` deliberately skips, so it never appears there.

### 6. Test it

Every real plugin's tests spin up a local `httptest.Server` that mimics
the real API's shape, rather than hitting the network -- see
`exampleplugin/plugin_test.go`, or any real plugin's own `_test.go` for
a fuller example (`homeassistant_test.go` fakes two endpoints; the
`msgraph*` plugins fake the Graph SDK's own base URL). Point your
plugin's `baseURL` field at `srv.URL` in the test, call `Configure` then
`Fetch`, assert on the returned rows. Test `Configure`'s validation and
a `Fetch`-before-`Configure` error case too.

### Optional: dynamic setup fields (Discoverable)

If a `SetupField`'s real choices only exist once an instance has live
credentials (e.g. "which of your calendars" can't be known before
OAuth), mark it `Dynamic: true` and implement
`plugindata.Discoverable`:

```go
type Discoverable interface {
    Discover(ctx context.Context, field string, cfg map[string]any) ([]DiscoveredOption, error)
}
```

This is optional -- most plugins don't implement it. See
`internal/plugins/data/homeassistant/homeassistant.go` (a plain
`api_key` plugin using it for its entity picker) or
`internal/plugins/data/msgraphcalendar/` (an OAuth2 plugin using it for
its calendar picker) for real examples, and
[Discovery](../architecture.md#oauth2-built) for how the admin API
decides when to call it.

---

## Writing a UI plugin

A UI plugin is a plain object matching `UIPlugin<TData>`
(`web/src/shared/types/plugin.ts`):

```ts
export interface UIPlugin<TData = unknown> {
  id: string;
  name: string;
  dataShape: string;
  defaultSize: GridSize;
  minSize: GridSize;
  maxSize?: GridSize;
  configSchema?: Record<string, ConfigField>;
  component: ComponentType<WidgetProps<TData>>;
}
```

### 1. Scaffold the folder

```
web/src/plugins/your-widget/
  YourWidget.tsx
  YourWidget.css
```

One folder per widget, matching every existing plugin under
`web/src/plugins/`.

### 2. Declare the row type

Define an interface for one row of `GET /api/data/{shape}` matching that
shape's columns verbatim (snake_case, same names as the Go struct's
`db:"..."` tags -- there's no generated/shared type between Go and
TypeScript, you write this by hand). See `example-widget/ExampleWidget.tsx`'s
`ExampleRow` for `weather_current`'s shape, or any other widget for a
different one.

### 3. Write the component

```tsx
function YourWidgetComponent({ data, config, size, theme, pluginInstanceId }: WidgetProps<YourRow>) {
  // ...
}
```

- `data` is the shape's rows, already fetched (empty array if none yet
  -- always handle that; it's a fresh instance's normal state, not an
  error).
- `config` is `Record<string, unknown>` -- cast it to your own `Config`
  interface matching `configSchema` below.
- `size` is the card's current grid units (`{w, h}`) -- the one hint a
  widget gets about how much room it has; there's no ResizeObserver or
  pixel measurement. Drop secondary details at small sizes (see
  `weather-current`'s `compact` handling) rather than overflowing.
- `theme` is the resolved `ThemeTokens` (already merged with the card's
  own `theme_override`, if any) -- render your own full card chrome from
  it via `cardStyle()` (`web/src/plugins/shared/cardStyle.ts`); there's
  no separate wrapping `Card` component.
- `pluginInstanceId` is the card's own `data_plugin_instance_id`
  (`null` if unset) -- most widgets ignore it, since `data` already
  carries their one shape's rows. It exists for a widget that needs a
  *second*, related shape from the same instance, fetched itself via the
  display's own `useShapeData` hook (see `mullet-calendar-agenda` or
  `mullet-task-list` for real examples of this pattern).

### 4. Export the manifest

```tsx
export const yourWidgetPlugin: UIPlugin<YourRow> = {
  id: 'your-widget',           // 'mullet-' prefix is first-party-only, see below
  name: 'Your Widget',
  dataShape: 'weather_current', // or '' if the widget needs no data at all, like mullet-clock
  defaultSize: { w: 4, h: 3 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 6 },
  configSchema: {
    someField: { type: 'toggle', label: 'Show something', default: true },
  },
  component: YourWidgetComponent,
};
```

`id` must be unique across every registered UI plugin. By convention,
first-party plugins are prefixed `mullet-` (`mullet-weather-current`) to
leave the unprefixed namespace free for community plugins -- leave the
prefix off your own.

### 5. Register it

Add it to `web/src/plugins/registry.ts`'s `uiPlugins` array (this is
what makes `getUIPlugin(id)` resolve it for a real card), and, if you
want it selectable in the Designer today,
`web/src/admin/pages/DesignerPage.tsx`'s hand-maintained `PALETTE` list
(there's no server-side UI plugin registry yet -- see that file's own
comment on why the palette is separate).

Neither step happens in `example-widget/`, so it stays invisible to the
real app.

### 6. Verify it live

Type-checking and `npm run lint` don't verify a widget actually renders
correctly -- start the dev server, add a card using it in the Designer,
and check it against real (or manually seeded) data before considering
it done. See any recent plugin's own commit history for the level of
live-verification this project expects.

---

## Adding a new data shape

Shapes are framework-owned, not plugin-owned -- adding one touches a
few files, all in `internal/`:

1. **Go struct** -- `internal/shapes/yourshape.go`, a plain struct with
   `db:"..."` tags matching your SQL columns 1:1 (pointer types for
   nullable columns, e.g. `*string`, `*float64`). Every shape carries
   `ID`/`PluginInstanceID` at minimum. See any existing file in
   `internal/shapes/` for the pattern -- `weather.go` for a simple
   standalone shape, `events.go`/`tasks.go` for one with an associated
   entity-discovery metadata table.
2. **Migration** -- a new numbered file in `internal/db/migrations/`
   (`CREATE TABLE shape_yourshape (...)`, `plugin_instance_id INTEGER
   NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE`,
   `fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`, a composite
   `PRIMARY KEY (id, plugin_instance_id)`). Migrations are applied in
   filename order and tracked so each runs exactly once (`internal/db`'s
   `Migrate`) -- never edit an already-merged migration, add a new one.
3. **Writer** -- `internal/db/store.go`: an explicit `shapeWriter` function
   (there's no reflection-based generic writer; every shape gets its own,
   even though most follow the same **Replace** pattern -- delete this
   instance's existing rows, insert the fresh set, in one transaction)
   plus an entry in the `shapeWriters` map keyed by your shape's string
   name.
4. **Read path** -- `internal/db/data_api.go`'s `shapeTables` map: add
   `"yourshape": "shape_yourshape"` so `GET /api/data/yourshape` (and the
   admin API) can read it. This map is intentionally wider than
   `shapeWriters` -- a shape's table can be readable before any plugin
   writes to it.

If your shape needs entity discovery (a plugin can't resolve a real
foreign key itself, e.g. "which calendar does this event belong to") --
see `events`/`tasks`'s **Upsert** strategy and the `calendars`/
`task_lists` metadata tables in
[Write path](../architecture.md#write-path) instead of the simpler
Replace pattern above.

---

## Testing and verification checklist

Before considering a plugin done:

- [ ] `go build ./...`, `go vet ./...`, `go test -race ./...` all pass
- [ ] `npm run build`, `npm run lint` pass (UI plugins)
- [ ] Your plugin's own tests cover `Configure` validation and a
      realistic `Fetch` against a fake server (data plugins)
- [ ] Live-verified: added a real instance through the admin UI, saw it
      sync successfully, and (for a UI plugin) saw it render correctly
      on an actual display with real or seeded data -- not just
      "it compiles"

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the rest of the PR
process.

# Mullet Admin UI — implementation reference

Two static HTML mockups accompany this doc, ready to open directly or hand to a coding session:

- `admin-ui-dashboard.html` — the Admin UI shell + Dashboard page
- `admin-ui-designer.html` — the Designer (drag-and-drop screen builder)

Both are plain HTML with inline styles and one Google Fonts link, no build step, so they open straight in a browser. They're layout/visual references, not production code: real components will need actual state, drag behavior, routing, etc. (see the project's `overview.md` / `architecture_1.md` for the underlying data model — plugin instances, displays/screens/cards, themes).

Live, still-editable version of both (canvas view, side by side): https://claude.ai/code/artifact/0a667bf0-bd18-49d2-a7c8-3b06470bc90a

## Scope note

This covers the **server-side admin app only** — not the wall-mounted Display renderer (`/display/{slug}`), which has its own separate theming system (frosted glass / solid widget styles, per-display theme cascade) already decided elsewhere and out of scope here.

## Design tokens (working draft — not confirmed)

```css
--bg-app: #F7F4EF;       /* page background, warm off-white */
--surface: #FFFFFF;      /* cards, sidebar-active state */
--surface-alt: #FBF8F2;  /* nested rows, subtle fills */
--ink: #2A2320;          /* primary text, warm near-black */
--ink-soft: #8A7E73;     /* secondary text, labels */
--border: #E7E0D6;
--accent: oklch(70% 0.16 45);       /* coral/tangerine, primary actions */
--accent-deep: oklch(52% 0.15 40);  /* accent on light backgrounds, icons */
--ok: #4E8E76;    /* synced / online status */
--warn: #C08A2E;  /* retrying / degraded status */
--error: #B5493C; /* destructive actions */
```

Typography: **Space Grotesk** (600–700) for headings/numbers, **Manrope** (400–800) for everything else. Both via Google Fonts.

Shape language: 10–16px border radius depending on element size (small chips ~8-9px, cards 12-16px), 1px hairline borders in `--border`, soft low-opacity shadows only on elevated/floating elements (stat cards flat, popovers/selected states get shadow).

These tokens are a starting point pulled from an earlier direction exploration — Ryan hasn't explicitly signed off on the palette/type yet, so treat as provisional until confirmed.

## Screen 1 — Admin UI (Dashboard)

Standard sidebar app shell, reused across all Admin UI pages (Data Plugins, Displays, Themes, Clients, Settings — only Dashboard is mocked in full).

**Sidebar** (232px, fixed): logo mark + wordmark, nav list (Dashboard, Data Plugins, Displays, Designer, Themes, Clients, Settings), user chip pinned to bottom. "Designer" is visually distinct (dashed border, chevron) since it's a launcher into a different screen mode rather than a page — open question below on whether it belongs here at all.

**Dashboard content**:
- Header row: page title + subtitle, "All systems normal" status chip (right-aligned)
- 4-up stat card row: Uptime, Data Plugins (active count), Displays (count), Active Clients
- Two-column split below: left = **Plugin Status** list (icon, name, source detail, synced/retrying status dot), right = **Displays** list (thumbnail swatch, name, screen count + online state, "Open in Designer" link) with a **Recent Activity** feed underneath

## Screen 2 — Designer

Deliberately different chrome from the rest of Admin UI — full-bleed, toolbar-driven, three-pane layout like a design tool rather than a sidebar app.

**Top toolbar** (58px): back arrow, display name + breadcrumb chevron, screen tabs (Main / Detail / +), spacer, theme swatch + name, Preview button, Save button (accent-filled).

**Left: Widget palette** (224px) — draggable list of available UI plugins (Calendar Agenda, Weather Forecast, Meal Plan, Task List, Home Status, Package Tracker, Clock), each with an icon + label.

**Center: Grid canvas** — dot/line grid background, placed cards laid out per the display's current `cards` rows (col/row/width/height from the schema in `architecture_1.md`). One card shown selected: accent outline + 4 corner resize handles, to communicate the direct-manipulation interaction.

**Right: Inspector** (280px) — bound to whichever card is selected. Fields shown: Data Source (select, populated from configured plugin instances), a numeric stepper (Days Shown), two toggles (a `configSchema`-driven boolean, and Theme Override), and a destructive "Remove card" action at the bottom.

## Open questions (flag these before building for real)

1. **Nav placement of Designer** — is it a peer top-level area (as mocked, its own sidebar item), or should it only be reachable by clicking "Open in Designer" from a specific display in the Displays list (removing it from the sidebar entirely)?
2. **Color/type direction** — the coral/warm-neutral palette above was carried over from an earlier exploration and hasn't been explicitly approved. Worth a deliberate pass before it goes into real components.
3. **Not yet mocked**: first-run setup wizard, login, manifest-driven plugin setup form, theme editor, client pairing/approval flow, settings page.

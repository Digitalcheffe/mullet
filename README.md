<p align="center">
  <img src="docs/mullet1.png" alt="Mullet" width="360">
</p>

> Your family calendar, weather, and whatever else is worth a glance — always up on a TV, tablet, or spare display, without a subscription.

Mullet is a self-hosted digital signage platform for the home. Point a spare TV, a wall-mounted tablet, or a Raspberry Pi + monitor at it, and it becomes an always-on household information display: what's on the calendar today, whose birthday is coming up, the weather, what's for dinner, package deliveries — whatever's relevant to where that screen hangs.

---

## A Note on How This Is Built

Mullet is built with **[Claude Code](https://claude.ai/code)**. Every feature goes through a tracked GitHub issue, every change ships as its own PR referencing that issue, and nothing merges without passing the full build/test suite and a live check in an actual browser. Read the [issue history](https://github.com/Digitalcheffe/mullet/issues) and [PRs](https://github.com/Digitalcheffe/mullet/pulls) — the process is the proof.

Mullet is built on the shoulders of great open-source work — [see the full list of projects credited below](#built-on).

---

## What it does

Mullet is two things in one Docker image:

- **A display server** — a grid-based layout designer (drag-and-drop or size-preset), a library of home-screen widgets, and per-screen/per-card theming, served to any device with a browser. Add as many screens around the house as you want; each one is just a browser tab.
- **A data layer** — a scheduler that polls calendars, weather, smart-home devices, and more on a timer and writes the results to SQLite, so every widget renders from local data instead of hitting an API on every page load.

Everything lives in one image, writes to one `/data` folder, and runs on a Raspberry Pi or a full home server equally well.

---

## Core features

- **The calendar is the centerpiece** — an agenda list, a week-view grid, or both. Merge multiple sources (Family + Birthdays + Holidays, personal + a partner's) onto one card, color-coded by source, with today's already-passed events greyed out.
- **Drag-and-drop layout designer** — a simple size-preset grid for quick layouts, or a free-form pixel grid for full control, per screen.
- **A real widget library** — weather, upcoming birthdays, countdowns, meal plans, task/chore lists, a quote of the day, moon phase, sticky notes, and more.
- **Plug in what you already use** — Home Assistant, package tracking, media now-playing, an ICS calendar feed, and Microsoft Graph (Outlook calendar + To Do) via OAuth2/PKCE.
- **A real plugin system** — both the data layer and the widget layer are pluggable; see [`docs/plugin-development.md`](docs/plugin-development.md) to add your own.
- **Household-ready admin** — multiple accounts, optional two-factor authentication (TOTP), email/webhook notification preferences, and SMTP-backed password reset.
- **Single binary, single data directory** — mount `/data`, done.

---

## Quick Start

```bash
git clone https://github.com/Digitalcheffe/mullet.git
cd mullet
docker compose up -d
```

`docker-compose.yml` builds the image locally (there's no published image yet) and mounts `./data` for the database, uploads, and logs.

Open `http://localhost:8080/admin` to run first-time setup (create the admin account), then `http://localhost:8080/register` from any display device to add it to a screen.

Everything Mullet needs to keep — the database, uploaded images, and logs — lives under `/data`, so rebuilding and restarting the container (`git pull && docker compose up -d --build`) never loses it.

---

## Configuration

### Environment variables

| Variable | Description | Default | Required |
| --- | --- | --- | --- |
| `PORT` | HTTP port — drives both the container's listen port and the host mapping | `8080` | No |
| `DB_PATH` | SQLite database file location | `/data/mullet.db` | No |
| `UPLOADS_DIR` | Uploaded files (theme backgrounds, etc.) | `/data/uploads` | No |
| `LOG_PATH` | Log file location — its directory is created automatically if missing | `/data/logs/mullet.log` | No |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM_ADDRESS` / `SMTP_TLS_MODE` | Outgoing mail for password reset and notifications. Seeds the setting on first boot only — once configured via Settings, the admin UI is the permanent source of truth | — | No |
| `CORS_ORIGINS` | Comma-separated allowed origins, if the admin UI is served from a different origin than the API | — | No |
| `AUTH_DISABLED` | **Local development only.** Skips login entirely. Never set this in a real deployment | `false` | No |

No secret needs to be generated or supplied by hand — the JWT signing key is created automatically on first boot and persisted in the database.

### In-app settings

Runtime configuration lives in **Settings** inside the app — no env vars needed:

- Outgoing mail (SMTP), once you'd rather manage it there than via env vars
- Log file destination
- User accounts, two-factor authentication, notification preferences
- Themes, background images, per-screen font overrides
- Every data source (calendars, weather, Home Assistant, etc.) and widget placement

---

## Widget & data source library

| Category | Widgets |
| --- | --- |
| Calendar | Agenda list · Week view (multi-source merge, per-calendar color coding) |
| Home | Upcoming birthdays · Countdown · Meal plan · Task/chore list · Text/notes |
| Ambient | Weather (current + forecast) · Quote of the day · Moon phase · Clock |
| Live status | Home Assistant devices · Server/infrastructure health · Media now-playing · Package tracker |

| Data source | Notes |
| --- | --- |
| OpenWeatherMap / Open-Meteo | Weather |
| ICS calendar feed | Any calendar that publishes a standard `.ics` URL |
| Microsoft Graph | Outlook calendar + To Do, via OAuth2/PKCE (no client secret needed for a personal account) |
| Home Assistant | Devices, sensors, climate |

Custom data and widget plugins are both first-class — see [`docs/plugin-development.md`](docs/plugin-development.md).

---

## Stack

| Layer | Choice |
| --- | --- |
| Backend | Go — single binary, cross-compiles for ARM (Raspberry Pi) without a C toolchain |
| Database | SQLite — single file, schema-migrated on startup, zero ops |
| Frontend | React 19 + TypeScript + Vite |
| Deployment | Single multi-stage Docker image |

3-stage Docker build: Go builder → Node/Vite builder → `alpine` final image. See [`architecture.md`](architecture.md) for how the whole system fits together.

---

## Built On

Mullet would not exist without these open-source projects:

| Project | Role |
| --- | --- |
| [Go](https://go.dev/) | Backend runtime |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Embedded database — pure Go, no cgo, so the server cross-compiles for ARM cleanly |
| [golang-jwt/jwt](https://github.com/golang-jwt/jwt) | JWT authentication |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | Password hashing (bcrypt) |
| [arran4/golang-ical](https://github.com/arran4/golang-ical) | ICS calendar feed parsing |
| [teambition/rrule-go](https://github.com/teambition/rrule-go) | Recurring-event (RRULE) expansion |
| [microsoftgraph/msgraph-sdk-go](https://github.com/microsoftgraph/msgraph-sdk-go) | Outlook calendar + To Do integration |
| [React](https://react.dev/) + [Vite](https://vitejs.dev/) | Frontend framework and build tool |
| [React Router](https://reactrouter.com/) | Client-side routing |
| [react-grid-layout](https://github.com/react-grid-layout/react-grid-layout) | The Designer's drag/resize/collision-detection grid |
| [qrcode.react](https://github.com/zpao/qrcode.react) | QR code rendering for two-factor auth enrollment |

---

## Contributing

Open an issue first so we can align on approach before you build. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for code style, testing conventions, and the PR process.

---

## Status

Actively developed, pre-1.0. The core display app, admin UI, and a wide plugin set are built and working; see the [issue tracker](https://github.com/Digitalcheffe/mullet/issues) for what's in progress.

---

## License

[MIT](LICENSE)

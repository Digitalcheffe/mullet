# Contributing to Mullet

Mullet is a self-hosted widget dashboard: a Go server (data plugins,
SQLite, REST API, admin UI) and a React frontend (admin app + display
renderer). Start with [`architecture.md`](architecture.md) for how the
whole system actually fits together -- it's kept current as the source
of truth, ahead of the original design proposal in
[`docs/architecture_1.md`](docs/architecture_1.md). Adding a plugin?
See [`docs/plugin-development.md`](docs/plugin-development.md) instead
of reverse-engineering one from an existing plugin.

## Code style

- **Go**: `gofmt` is non-negotiable -- run it (or let your editor) before
  committing. Errors are wrapped with context (`fmt.Errorf("doing X: %w", err)`),
  not swallowed or logged-and-ignored. Table-driven tests where the
  cases are genuinely uniform; separate `Test...` functions where they're
  not (most of this codebase's tests are the latter -- one function per
  behavior, not one giant table).
- **TypeScript/React**: function components, hooks, no class components.
  `npm run lint` (oxlint) and `tsc -b` (via `npm run build`) both need to
  pass. Plain CSS per component (`Widget.tsx` + `Widget.css`), no CSS-in-JS,
  no general component library -- the one exception is `react-grid-layout`
  for the Designer's drag/resize grid specifically.
- **Comments**: explain *why*, not *what* -- a well-named function or
  variable already says what it does. Write a comment when there's a
  non-obvious constraint, a deliberate trade-off, or a reason something
  *isn't* done the more obvious way (grep this codebase for "-- " as a
  rough example of the density and style this project already uses).
  Don't restate the code in prose above it.
- **No dead code, no commented-out code, no speculative abstraction**
  for a single current caller. If something's unused, delete it rather
  than leaving it "just in case."

## Before opening a PR

- `go build ./...`, `go vet ./...`, `go test -race ./...` all pass.
- `npm run build`, `npm run lint` (in `web/`) both pass.
- Any UI-visible change has been checked in an actual running browser
  against the real (or realistically seeded) app -- type-checking and a
  green test suite verify correctness, not that a feature actually
  looks and works right. This matters most for anything touching the
  display renderer, the admin UI, or a UI plugin's rendering.
- `architecture.md` is updated in the same PR if the change makes
  something in it inaccurate (a section moves from Planned to Built, a
  described behavior changes, a new table/endpoint/component exists).
  Docs drifting out of sync with the code is treated as a bug in the
  PR, not a follow-up.

## PR process

- One issue, one branch, one PR: branch as `issue-<N>-<short-slug>`
  (e.g. `issue-32-plugin-docs`), PR title `<Imperative summary> (issue
  #N)`, PR body ending with `Closes #N` so merging auto-closes it.
  Squash-merge is this repo's convention -- keep the commit history on
  `main` one entry per shipped change, not per intermediate commit on
  the branch.
- Commit/PR descriptions explain **why**, matching the comment style
  above -- what problem this solves and any non-obvious trade-off,
  not just a restatement of the diff.
- Scope creep: if you notice something else worth fixing while working
  (a stale doc claim, an unrelated bug), either fix it in the same PR
  when it's small and directly touches code you're already changing, or
  file a separate issue for it. Don't let an issue's PR balloon into an
  unrelated cleanup pass.

## Testing conventions

- A data plugin's tests run against a local `httptest.Server` faking
  the real API's shape -- never against the real network. See any
  existing plugin's `_test.go` (`internal/plugins/data/*/`.).
- Prefer one assertion-focused `Test...` function per behavior over a
  single sprawling test with many unrelated checks -- easier to read
  the failure, easier to see what's actually covered.
- A migration that changes row counts or seeded data will break
  `internal/db/sqlite_test.go`'s own assertions about them (schema
  migration count, seeded theme count) -- update those in the same PR,
  don't skip or delete the test.

## Getting help

Open an issue for anything not already covered by an existing one in
this repo's tracker -- a bug, a design question, a feature that isn't
already on the roadmap. There's no separate chat/forum for this project
yet.

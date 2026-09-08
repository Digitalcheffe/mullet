-- Dedicated client app registration (issue #29). See docs/architecture_1.md
-- Client for the full registration/polling/offline-mode design.
--
-- `status` only ever holds the admin's own lifecycle decision --
-- 'pending' (awaiting approval), 'approved', or 'rejected'. Whether an
-- approved client is currently online is *not* stored here: a client
-- that's actually offline has no way to tell the server so, which makes
-- a stored "offline" status meaningless. The API layer instead derives
-- online/offline from how recently last_seen_at was touched.
CREATE TABLE clients (
    id              INTEGER PRIMARY KEY,
    client_id       TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    display_id      INTEGER REFERENCES displays(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    last_seen_at    DATETIME,
    offline_mode    TEXT NOT NULL DEFAULT 'offline_screen',
    platform        TEXT,
    app_version     TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Per-display offline screen HTML, admin-configured. NULL means the
-- client falls back to a server-generated default (clock + display
-- name, no admin setup required).
ALTER TABLE displays ADD COLUMN offline_screen_html TEXT;

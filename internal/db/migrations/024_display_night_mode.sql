-- Issue #91: a per-display night-mode schedule that dims the display
-- (a CSS brightness filter, the "simplest version" the issue calls
-- out) during a configurable overnight window. Disabled and unset by
-- default for every existing display.
ALTER TABLE displays ADD COLUMN night_mode_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE displays ADD COLUMN night_start TEXT;
ALTER TABLE displays ADD COLUMN night_end TEXT;
ALTER TABLE displays ADD COLUMN night_brightness REAL NOT NULL DEFAULT 0.4;

-- Issue #171: a per-display toggle for the screen-fade/card-entrance
-- animations added in issue #86, independent of the OS-level
-- prefers-reduced-motion setting those already respect. Enabled by
-- default -- matches every existing display's current behavior.
ALTER TABLE displays ADD COLUMN transitions_enabled INTEGER NOT NULL DEFAULT 1;

-- Per-display toggle for the fixed top/bottom bar zones (clock/date,
-- now-playing, alerts, screen dots -- see architecture.md "Grid System").
-- Both default on: a freshly created display looks complete without the
-- admin having to configure anything.
ALTER TABLE displays ADD COLUMN show_top_bar INTEGER NOT NULL DEFAULT 1;
ALTER TABLE displays ADD COLUMN show_bottom_bar INTEGER NOT NULL DEFAULT 1;

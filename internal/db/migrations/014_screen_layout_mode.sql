-- Simple mode (issue #73) is the new default going forward, and for
-- every screen that already exists -- layout_mode only changes what the
-- Designer *offers* (grid density, resize handles vs. S/M/L presets),
-- never the card x/y/w/h values already stored, so nothing already
-- placed moves or looks different on the live display.
ALTER TABLE screens ADD COLUMN layout_mode TEXT NOT NULL DEFAULT 'simple';

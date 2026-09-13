-- Issue #162: an optional aspect-ratio preset for a screen (e.g.
-- "16:9", "9:16", "4:3", "21:9"), null meaning "Custom" -- today's
-- free-form behavior, unchanged. Purely a Designer-canvas preview
-- hint (CSS aspect-ratio on the canvas), not a pixel-dimension lock --
-- columns/row_height/gap stay independently configurable regardless.
ALTER TABLE screens ADD COLUMN aspect_ratio TEXT;

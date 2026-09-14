-- Issue #175: an entity's own Home Assistant-reported icon (an
-- already-cached, same-origin URL like "/icons/water-percent.svg",
-- resolved server-side from its "mdi:xxx" attribute -- see
-- internal/mdiicons), for the Home Status/Home Assistant Entity
-- widgets to prefer over their own hardcoded domain-based icon set.
-- NULL means no icon reported, or it didn't resolve to a known one --
-- the widgets fall back to their existing icon set either way.
ALTER TABLE shape_home_devices ADD COLUMN icon TEXT;

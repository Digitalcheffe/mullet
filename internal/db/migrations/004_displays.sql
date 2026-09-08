-- Visual theme token sets. Referenced by a display as its base theme,
-- and overridable per card (see cards.theme_override).
CREATE TABLE themes (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    tokens      TEXT NOT NULL DEFAULT '{}',
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- A named physical output, routed at /display/{slug}. Rotates through
-- its screens on rotation_seconds.
CREATE TABLE displays (
    id                INTEGER PRIMARY KEY,
    name              TEXT NOT NULL,
    slug              TEXT UNIQUE NOT NULL,
    theme_id          INTEGER REFERENCES themes(id),
    rotation_seconds  INTEGER NOT NULL DEFAULT 30,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- A page within a display; a display must rotate through at least one.
-- Each screen owns its own grid (columns/row_height/gap in pixels) --
-- screens are not required to share a grid with their siblings.
CREATE TABLE screens (
    id          INTEGER PRIMARY KEY,
    display_id  INTEGER NOT NULL REFERENCES displays(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    columns     INTEGER NOT NULL DEFAULT 16,
    row_height  INTEGER NOT NULL DEFAULT 40,
    gap         INTEGER NOT NULL DEFAULT 8,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_screens_display ON screens(display_id);

-- A positioned UI plugin on a screen's grid. data_plugin_instance_id is
-- nullable and ON DELETE SET NULL -- a card whose data source gets
-- removed stays in place (still has a UI plugin, position, and config)
-- rather than vanishing or blocking the delete; the admin can reassign
-- it. ui_plugin_id has no FK: UI plugins are compiled into the frontend
-- bundle, not tracked in a server-side table.
CREATE TABLE cards (
    id                       INTEGER PRIMARY KEY,
    screen_id                INTEGER NOT NULL REFERENCES screens(id) ON DELETE CASCADE,
    ui_plugin_id             TEXT NOT NULL,
    data_plugin_instance_id  INTEGER REFERENCES data_plugin_instances(id) ON DELETE SET NULL,
    x                        INTEGER NOT NULL,
    y                        INTEGER NOT NULL,
    w                        INTEGER NOT NULL,
    h                        INTEGER NOT NULL,
    config                   TEXT NOT NULL DEFAULT '{}',
    theme_override           TEXT,
    created_at               DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cards_screen ON cards(screen_id);

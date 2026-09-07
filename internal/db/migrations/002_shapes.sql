-- Enabled data plugins and their configuration. Every shape table below
-- scopes its rows to a plugin instance via ON DELETE CASCADE.
CREATE TABLE data_plugin_instances (
    id              INTEGER PRIMARY KEY,
    plugin_id       TEXT NOT NULL,
    instance_name   TEXT NOT NULL,
    config          TEXT NOT NULL DEFAULT '{}',
    enabled         INTEGER NOT NULL DEFAULT 1,
    refresh_seconds INTEGER NOT NULL,
    last_fetch_at   DATETIME,
    last_error      TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Metadata tables: discovered entities for shapes that have a parent
-- concept (a calendar, a task list). Kept out of the shape rows so the
-- admin UI can query and toggle them directly.
CREATE TABLE calendars (
    id                 INTEGER PRIMARY KEY,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    name               TEXT NOT NULL,
    color              TEXT,
    enabled            INTEGER NOT NULL DEFAULT 1,
    UNIQUE(plugin_instance_id, external_id)
);

CREATE TABLE task_lists (
    id                 INTEGER PRIMARY KEY,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    name               TEXT NOT NULL,
    enabled            INTEGER NOT NULL DEFAULT 1,
    UNIQUE(plugin_instance_id, external_id)
);

-- events (shapes.CalendarEvent)
CREATE TABLE shape_events (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    calendar_id        INTEGER NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    start              DATETIME NOT NULL,
    end                DATETIME,
    all_day            INTEGER NOT NULL DEFAULT 0,
    location           TEXT,
    description        TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_events_calendar ON shape_events(calendar_id);
CREATE INDEX idx_events_start ON shape_events(start);

-- tasks (shapes.Task)
CREATE TABLE shape_tasks (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    task_list_id       INTEGER NOT NULL REFERENCES task_lists(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    completed          INTEGER NOT NULL DEFAULT 0,
    due_date           TEXT,
    sort_order         INTEGER NOT NULL DEFAULT 0,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_tasks_list ON shape_tasks(task_list_id);

-- weather_current (shapes.WeatherCurrent) -- no metadata table, standalone
CREATE TABLE shape_weather_current (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    temp               REAL NOT NULL,
    feels_like         REAL,
    condition          TEXT NOT NULL,
    icon               TEXT NOT NULL,
    humidity           INTEGER,
    high               REAL,
    low                REAL,
    sunrise            TEXT,
    sunset             TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);

-- weather_forecast (shapes.WeatherForecast) -- no metadata table, standalone
CREATE TABLE shape_weather_forecast (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    date               TEXT NOT NULL,
    high               REAL NOT NULL,
    low                REAL NOT NULL,
    condition          TEXT NOT NULL,
    icon               TEXT NOT NULL,
    precip_chance      INTEGER,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);
CREATE INDEX idx_forecast_date ON shape_weather_forecast(date);

-- home_devices (shapes.HomeDevice) -- custom contract, no metadata table
CREATE TABLE shape_home_devices (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    area               TEXT,
    device_type        TEXT NOT NULL,
    state              TEXT NOT NULL,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);

-- packages (shapes.Package) -- custom contract, no metadata table
CREATE TABLE shape_packages (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    carrier            TEXT NOT NULL,
    description        TEXT,
    status             TEXT NOT NULL,
    eta                DATETIME,
    tracking_url       TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);

-- infrastructure (shapes.InfraService) -- custom contract, no metadata table
CREATE TABLE shape_infrastructure (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    status             TEXT NOT NULL,
    cpu_percent        REAL,
    memory_percent     REAL,
    disk_percent       REAL,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);

-- media_status (shapes.MediaStatus) -- custom contract, no metadata table
CREATE TABLE shape_media_status (
    id                 TEXT NOT NULL,
    plugin_instance_id INTEGER NOT NULL REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    player_name        TEXT NOT NULL,
    is_playing         INTEGER NOT NULL DEFAULT 0,
    title              TEXT,
    artist             TEXT,
    album_art_url      TEXT,
    fetched_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, plugin_instance_id)
);

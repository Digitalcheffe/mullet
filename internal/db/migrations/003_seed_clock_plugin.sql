-- Auto-provision the clock plugin so a fresh install has a working data
-- plugin running out of the box, with no admin UI configuration needed.
INSERT INTO data_plugin_instances (plugin_id, instance_name, refresh_seconds)
VALUES ('clock', 'Clock', 60);

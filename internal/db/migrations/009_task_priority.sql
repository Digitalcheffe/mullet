-- Priority for the task-list UI plugin (issue #26). "low"/"normal"/"high",
-- matching Microsoft Graph's own Importance values -- the only producer
-- so far. A plugin with no concept of priority leaves it at the default.
ALTER TABLE shape_tasks ADD COLUMN priority TEXT NOT NULL DEFAULT 'normal';

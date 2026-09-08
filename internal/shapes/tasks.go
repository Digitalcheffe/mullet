package shapes

// Task represents a to-do item. Written by task data plugins, consumed by
// task list UI plugins.
// Related metadata: task_lists table (name, enabled).
type Task struct {
	ID               string  `db:"id"`
	PluginInstanceID int     `db:"plugin_instance_id"`
	TaskListID       int     `db:"task_list_id"`
	Title            string  `db:"title"`
	Completed        bool    `db:"completed"`
	DueDate          *string `db:"due_date"`
	SortOrder        int     `db:"sort_order"`
	// Priority is "low", "normal", or "high" (matching Microsoft Graph's
	// own Importance values, the only producer so far) -- a plugin that
	// has no concept of priority just leaves it as the zero value, which
	// the writer treats the same as "normal".
	Priority string `db:"priority"`

	// TaskListExternalID/TaskListName identify the source task list this
	// task belongs to. A plugin has no DB access to resolve a real
	// task_lists.id itself, so it sets these instead; the tasks writer
	// (internal/db/store.go) upserts a task_lists row per distinct
	// TaskListExternalID and fills in TaskListID before insert. Not
	// stored on shape_tasks itself -- write-time only. Same pattern as
	// CalendarEvent's CalendarExternalID/CalendarName.
	TaskListExternalID string
	TaskListName       string
}

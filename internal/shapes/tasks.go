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
}

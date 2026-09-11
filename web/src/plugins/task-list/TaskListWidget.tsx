import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { useShapeData } from '../../display/useShapeData';
import './TaskListWidget.css';

// One row from GET /api/data/tasks -- field names match the shape_tasks
// columns verbatim.
export interface TaskRow {
  id: string;
  task_list_id: number;
  title: string;
  completed: number; // SQLite INTEGER 0/1, not a JSON boolean
  due_date: string | null;
  sort_order: number;
  priority: string; // "low" | "normal" | "high"
  fetched_at: string;
}

// One row from GET /api/data/task_lists -- same entity-discovery
// metadata pattern as calendar-agenda's "calendars" lookup.
interface TaskListRow {
  id: number;
  plugin_instance_id: number;
  external_id: string;
  name: string;
  enabled: number;
}

interface Config {
  showCompleted?: boolean;
}

const PRIORITY_LABEL: Record<string, string> = { high: '!', normal: '', low: '' };

function formatDueDate(raw: string): string {
  const d = new Date(raw + 'T00:00:00Z');
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function TaskListComponent({ data, config, size, theme, pluginInstanceId }: WidgetProps<TaskRow>) {
  const cfg = config as Config;
  const showCompleted = cfg.showCompleted === true;
  const compact = size.h <= 3;

  const lists = useShapeData('task_lists', pluginInstanceId) as unknown as TaskListRow[];
  const listByID = new Map(lists.map((l) => [l.id, l]));

  const visible = showCompleted ? data : data.filter((t) => !t.completed);

  const byList = new Map<number, TaskRow[]>();
  for (const t of visible) {
    if (!byList.has(t.task_list_id)) byList.set(t.task_list_id, []);
    byList.get(t.task_list_id)!.push(t);
  }
  for (const list of byList.values()) {
    list.sort((a, b) => a.sort_order - b.sort_order);
  }

  const style = cardStyle(theme);

  if (visible.length === 0) {
    return (
      <div className="task-list-widget tl-empty mullet-card" style={style}>
        Nothing to do
      </div>
    );
  }

  return (
    <div className="task-list-widget mullet-card" style={style}>
      {[...byList.entries()].map(([listID, tasks]) => (
        <div className="tl-group" key={listID}>
          <div className="tl-group-header">{listByID.get(listID)?.name ?? 'Tasks'}</div>
          {tasks.map((t) => (
            <div className={`tl-task${t.completed ? ' tl-completed' : ''}`} key={t.id}>
              <span className={`tl-check${t.completed ? ' tl-check-done' : ''}`} style={t.completed ? { background: theme.accentColor, borderColor: theme.accentColor } : undefined} />
              <span className="tl-title">{t.title}</span>
              {t.priority === 'high' && (
                <span className="tl-priority-high" style={{ color: theme.accentColor }}>
                  {PRIORITY_LABEL.high}
                </span>
              )}
              {!compact && t.due_date && <span className="tl-due">{formatDueDate(t.due_date)}</span>}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

export const taskListPlugin: UIPlugin<TaskRow> = {
  id: 'mullet-task-list',
  name: 'Task List',
  dataShape: 'tasks',
  defaultSize: { w: 4, h: 6 },
  minSize: { w: 2, h: 3 },
  maxSize: { w: 8, h: 16 },
  configSchema: {
    showCompleted: { type: 'toggle', label: 'Show completed tasks', default: false },
  },
  component: TaskListComponent,
};

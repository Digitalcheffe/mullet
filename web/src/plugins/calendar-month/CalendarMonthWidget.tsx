import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { colorForID } from '../shared/idColor';
import { parseSQLDateTime } from '../shared/sqlDateTime';
import { localDayKey } from '../shared/dateMath';
import { useShapeData } from '../../display/useShapeData';
import type { EventRow } from '../calendar-agenda/CalendarAgendaWidget';
import './CalendarMonthWidget.css';

// Same entity-discovery shape calendar-agenda/calendar-week read -- see
// calendar-agenda's own CalendarRow doc comment.
interface CalendarRow {
  id: number;
  plugin_instance_id: number;
  external_id: string;
  name: string;
  color: string | null;
  enabled: number;
}

interface Config {
  startMonday?: boolean;
  // A month cell is far smaller than a week cell (issue #130 -- 5-6
  // rows instead of 1), so unlike calendar-week this needs an explicit
  // per-day cap rather than showing every event.
  maxPerDay?: number;
}

interface DayCell {
  date: Date;
  key: string;
  inMonth: boolean;
}

// 6 rows always, even for a month that only needs 5 -- a fixed grid
// height keeps the widget's own size (and every row's height) constant
// across months instead of jumping around as the active month changes.
function buildMonthGrid(today: Date, startMonday: boolean): DayCell[] {
  const monthStart = new Date(today.getFullYear(), today.getMonth(), 1);
  const dow = monthStart.getDay();
  const backToGridStart = startMonday ? (dow === 0 ? 6 : dow - 1) : dow;
  const gridStart = new Date(monthStart);
  gridStart.setDate(gridStart.getDate() - backToGridStart);

  const cells: DayCell[] = [];
  for (let i = 0; i < 42; i++) {
    const d = new Date(gridStart);
    d.setDate(d.getDate() + i);
    cells.push({ date: d, key: localDayKey(d), inMonth: d.getMonth() === today.getMonth() });
  }
  return cells;
}

function CalendarMonthComponent({ data, config, theme, pluginInstanceId }: WidgetProps<EventRow>) {
  const cfg = config as Config;
  const startMonday = cfg.startMonday ?? false;
  const maxPerDay = cfg.maxPerDay ?? 3;

  const calendars = useShapeData('calendars', pluginInstanceId) as unknown as CalendarRow[];
  const calendarByID = new Map(calendars.map((c) => [c.id, c]));

  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const todayKey = localDayKey(today);
  const cells = buildMonthGrid(today, startMonday);

  const byDay = new Map<string, EventRow[]>();
  for (const e of data) {
    const key = localDayKey(parseSQLDateTime(e.start));
    if (!byDay.has(key)) byDay.set(key, []);
    byDay.get(key)!.push(e);
  }
  for (const list of byDay.values()) {
    list.sort((a, b) => (a.all_day !== b.all_day ? b.all_day - a.all_day : a.start.localeCompare(b.start)));
  }

  const weekdayLabels = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(today);
    // Any Sunday-start week already anchors weekday index 0 at Sunday;
    // shifting by startMonday reuses the same 7 cells buildMonthGrid
    // already computed instead of re-deriving locale weekday order.
    d.setDate(d.getDate() - d.getDay() + i + (startMonday ? 1 : 0));
    return d.toLocaleDateString(undefined, { weekday: 'short' });
  });

  const style = cardStyle(theme);

  return (
    <div className="calendar-month-widget mullet-card" style={style}>
      <div className="cm-header">
        {today.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}
      </div>
      <div className="cm-weekday-row">
        {weekdayLabels.map((label, i) => (
          <span className="cm-weekday" key={i}>
            {label}
          </span>
        ))}
      </div>
      <div className="cm-grid">
        {cells.map((cell) => {
          const events = byDay.get(cell.key) ?? [];
          const shown = events.slice(0, maxPerDay);
          const overflow = events.length - shown.length;
          return (
            <div
              className={`cm-day${cell.inMonth ? '' : ' cm-day--outside'}${cell.key === todayKey ? ' cm-day--today' : ''}`}
              key={cell.key}
            >
              <span className="cm-day-num">{cell.date.getDate()}</span>
              <div className="cm-day-events">
                {shown.map((e) => {
                  const cal = calendarByID.get(e.calendar_id);
                  const color = cal?.color || colorForID(e.calendar_id);
                  return (
                    <span
                      className="cm-event"
                      key={e.id}
                      style={{ borderLeftColor: color }}
                      title={cal?.name ? `${e.title} (${cal.name})` : e.title}
                    >
                      {e.title}
                    </span>
                  );
                })}
                {overflow > 0 && <span className="cm-event-overflow">+{overflow} more</span>}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export const calendarMonthPlugin: UIPlugin<EventRow> = {
  id: 'mullet-calendar-month',
  name: 'Month View',
  dataShape: 'events',
  defaultSize: { w: 10, h: 8 },
  minSize: { w: 8, h: 6 },
  maxSize: { w: 16, h: 12 },
  // Same multi-calendar merge as calendar-week/calendar-agenda (issue
  // #97) -- a family month view is exactly the case that wants
  // Family + Birthdays + Holidays on one grid.
  supportsMultiDataSource: true,
  configSchema: {
    startMonday: { type: 'toggle', label: 'Week starts Monday', default: false },
    maxPerDay: { type: 'number', label: 'Max events shown per day', default: 3 },
  },
  component: CalendarMonthComponent,
};

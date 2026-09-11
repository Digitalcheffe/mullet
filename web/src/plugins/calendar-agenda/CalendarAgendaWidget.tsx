import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { colorForID } from '../shared/idColor';
import { useShapeData } from '../../display/useShapeData';
import './CalendarAgendaWidget.css';

// One row from GET /api/data/events -- field names match the
// shape_events columns verbatim.
export interface EventRow {
  id: string;
  calendar_id: number;
  title: string;
  start: string;
  end: string | null;
  all_day: number; // SQLite INTEGER 0/1, not a JSON boolean
  location: string | null;
  description: string | null;
  fetched_at: string;
}

// One row from GET /api/data/calendars -- the entity-discovery metadata
// writeEvents populates (see architecture.md "Write path"), read via the
// same generic shape mechanism as any other shape.
interface CalendarRow {
  id: number;
  plugin_instance_id: number;
  external_id: string;
  name: string;
  color: string | null;
  enabled: number;
}

interface Config {
  days?: number;
  showLocation?: boolean;
}

function parseSQLDateTime(raw: string): Date {
  // "YYYY-MM-DD HH:MM:SS" (UTC, per internal/db/data_api.go's
  // normalizeSQLValue) -- not directly Date-parseable without a "T" and
  // zone, so this makes it one. The resulting Date is a real instant;
  // grouping it into a calendar *day* still has to go through the
  // viewer's local timezone (localDayKey below), not UTC's, since a
  // kiosk display's "today" means the viewer's wall-clock today.
  return new Date(raw.replace(' ', 'T') + 'Z');
}

function localDayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function formatTime(d: Date): string {
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function CalendarAgendaComponent({ data, config, size, theme, pluginInstanceId }: WidgetProps<EventRow>) {
  const cfg = config as Config;
  const days = Math.max(1, cfg.days ?? 5);
  const showLocation = cfg.showLocation !== false;
  const compact = size.w <= 4;

  const calendars = useShapeData('calendars', pluginInstanceId) as unknown as CalendarRow[];
  const calendarByID = new Map(calendars.map((c) => [c.id, c]));

  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const dayList: { key: string; date: Date }[] = [];
  for (let i = 0; i < days; i++) {
    const d = new Date(today);
    d.setDate(d.getDate() + i);
    dayList.push({ key: localDayKey(d), date: d });
  }

  const byDay = new Map<string, EventRow[]>(dayList.map(({ key }) => [key, []]));
  for (const e of data) {
    const key = localDayKey(parseSQLDateTime(e.start));
    if (byDay.has(key)) byDay.get(key)!.push(e);
  }
  for (const list of byDay.values()) {
    list.sort((a, b) => (a.all_day !== b.all_day ? b.all_day - a.all_day : a.start.localeCompare(b.start)));
  }

  const style = cardStyle(theme);

  return (
    <div className="calendar-agenda-widget mullet-card" style={style}>
      {dayList.map(({ key, date }) => {
        const events = byDay.get(key)!;
        return (
          <div className="ca-day" key={key}>
            <div className="ca-day-header">
              {date.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: compact ? undefined : 'short' })}
            </div>
            {events.length === 0 ? (
              <div className="ca-empty">No events</div>
            ) : (
              events.map((e) => {
                const cal = calendarByID.get(e.calendar_id);
                const color = cal?.color || colorForID(e.calendar_id);
                return (
                  <div className="ca-event" key={e.id} style={{ borderLeftColor: color }}>
                    {e.all_day ? (
                      <span className="ca-badge" style={{ background: color }}>
                        All day
                      </span>
                    ) : (
                      <span className="ca-time" style={{ color: theme.accentColor }}>
                        {formatTime(parseSQLDateTime(e.start))}
                      </span>
                    )}
                    <span className="ca-title">{e.title}</span>
                    {showLocation && !compact && e.location && <span className="ca-location">{e.location}</span>}
                  </div>
                );
              })
            )}
          </div>
        );
      })}
    </div>
  );
}

export const calendarAgendaPlugin: UIPlugin<EventRow> = {
  id: 'mullet-calendar-agenda',
  name: 'Calendar Agenda',
  dataShape: 'events',
  defaultSize: { w: 6, h: 8 },
  minSize: { w: 3, h: 4 },
  maxSize: { w: 10, h: 16 },
  configSchema: {
    days: { type: 'number', label: 'Days to show', default: 5 },
    showLocation: { type: 'toggle', label: 'Show location', default: true },
  },
  component: CalendarAgendaComponent,
};

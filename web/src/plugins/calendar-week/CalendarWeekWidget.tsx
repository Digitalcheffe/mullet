import { useEffect, useState } from 'react';
import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { colorForID } from '../shared/idColor';
import { parseSQLDateTime } from '../shared/sqlDateTime';
import { localDayKey } from '../shared/dateMath';
import { useShapeData } from '../../display/useShapeData';
import type { EventRow } from '../calendar-agenda/CalendarAgendaWidget';
import './CalendarWeekWidget.css';

// Same entity-discovery shape calendar-agenda reads -- see its own
// CalendarRow doc comment.
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
}

function calendarInitial(name: string | undefined): string {
  return (name?.trim()?.[0] ?? '?').toUpperCase();
}

function CalendarWeekComponent({ data, config, theme, pluginInstanceId }: WidgetProps<EventRow>) {
  const cfg = config as Config;
  const startMonday = cfg.startMonday ?? false;

  const calendars = useShapeData('calendars', pluginInstanceId) as unknown as CalendarRow[];
  const calendarByID = new Map(calendars.map((c) => [c.id, c]));

  // Same independent tick as calendar-agenda's grey-out-past feature --
  // this widget shows the same "already passed today" treatment, and
  // needs it to update without waiting on the next data poll.
  const [, forceTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => forceTick((t) => t + 1), 30_000);
    return () => clearInterval(id);
  }, []);

  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const todayKey = localDayKey(today);

  // Sunday-start (dow 0) is JS's own Date.getDay() default; Monday-start
  // shifts the week back by however many days since the most recent
  // Monday.
  const dow = today.getDay();
  const backToWeekStart = startMonday ? (dow === 0 ? 6 : dow - 1) : dow;
  const weekStart = new Date(today);
  weekStart.setDate(weekStart.getDate() - backToWeekStart);

  const days: { key: string; date: Date }[] = [];
  for (let i = 0; i < 7; i++) {
    const d = new Date(weekStart);
    d.setDate(d.getDate() + i);
    days.push({ key: localDayKey(d), date: d });
  }

  const byDay = new Map<string, EventRow[]>(days.map(({ key }) => [key, []]));
  for (const e of data) {
    const key = localDayKey(parseSQLDateTime(e.start));
    if (byDay.has(key)) byDay.get(key)!.push(e);
  }
  for (const list of byDay.values()) {
    list.sort((a, b) => (a.all_day !== b.all_day ? b.all_day - a.all_day : a.start.localeCompare(b.start)));
  }

  function isPastEvent(e: EventRow, dayKey: string): boolean {
    if (dayKey !== todayKey || e.all_day) return false;
    return parseSQLDateTime(e.end ?? e.start).getTime() < now.getTime();
  }

  // Only the calendars actually represented this week -- see
  // calendar-agenda's identical reasoning.
  const visibleCalendarIds = new Set<number>();
  for (const list of byDay.values()) {
    for (const e of list) visibleCalendarIds.add(e.calendar_id);
  }
  const legend = Array.from(visibleCalendarIds).map((id) => {
    const cal = calendarByID.get(id);
    return { id, name: cal?.name ?? `Calendar ${id}`, color: cal?.color || colorForID(id) };
  });

  const style = cardStyle(theme);

  return (
    <div className="calendar-week-widget mullet-card" style={style}>
      <div className="cw-grid">
        {days.map(({ key, date }) => {
          const events = byDay.get(key)!;
          return (
            <div className={`cw-day${key === todayKey ? ' cw-day--today' : ''}`} key={key}>
              <div className="cw-day-header">
                <span className="cw-day-name">{date.toLocaleDateString(undefined, { weekday: 'short' })}</span>
                <span className="cw-day-num">{date.getDate()}</span>
              </div>
              <div className="cw-day-events">
                {events.map((e) => {
                  const cal = calendarByID.get(e.calendar_id);
                  const color = cal?.color || colorForID(e.calendar_id);
                  const past = isPastEvent(e, key);
                  return (
                    <div
                      className={`cw-event${past ? ' cw-event--past' : ''}`}
                      key={e.id}
                      style={{ borderLeftColor: color }}
                      title={cal?.name ? `${e.title} (${cal.name})` : e.title}
                    >
                      <span className="cw-event-meta">
                        <span className="cw-event-pill" style={{ background: color }}>
                          {calendarInitial(cal?.name)}
                        </span>
                        {!e.all_day && <span className="cw-event-time">{formatTime(parseSQLDateTime(e.start))}</span>}
                      </span>
                      <span className="cw-event-title">{e.title}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
      {legend.length > 1 && (
        <div className="cw-legend">
          {legend.map((l) => (
            <span className="cw-legend-item" key={l.id}>
              <span className="cw-legend-swatch" style={{ background: l.color }} />
              {l.name}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

function formatTime(d: Date): string {
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

export const calendarWeekPlugin: UIPlugin<EventRow> = {
  id: 'mullet-calendar-week',
  name: 'Week View',
  dataShape: 'events',
  defaultSize: { w: 8, h: 6 },
  minSize: { w: 6, h: 4 },
  maxSize: { w: 16, h: 10 },
  // Same multi-calendar merge as calendar-agenda (issue #97) -- a family
  // week view is exactly the case that wants Family + Birthdays +
  // Holidays on one grid.
  supportsMultiDataSource: true,
  configSchema: {
    startMonday: { type: 'toggle', label: 'Week starts Monday', default: false },
  },
  component: CalendarWeekComponent,
};

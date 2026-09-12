import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { parseSQLDateTime } from '../shared/sqlDateTime';
import type { EventRow } from '../calendar-agenda/CalendarAgendaWidget';
import './BirthdaysWidget.css';

interface Config {
  count?: number;
}

// Whole calendar days apart, in the viewer's local timezone -- matches
// calendar-agenda's own day-vs-instant distinction (see its
// localDayKey), since "3 days away" should count wall-clock days, not
// fractions of 24 hours that'd round oddly near midnight.
function daysUntil(from: Date, to: Date): number {
  const a = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const b = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  return Math.round((b.getTime() - a.getTime()) / 86_400_000);
}

function relativeDayLabel(n: number): string {
  if (n === 0) return 'Today';
  if (n === 1) return 'Tomorrow';
  return `In ${n} days`;
}

// UpcomingBirthdaysComponent reads the same shared "events" shape as
// Calendar Agenda -- issue #101 deliberately doesn't need its own data
// plugin or backend changes, just a card that filters/sorts a card's
// already-bound source (e.g. a dedicated ICS birthdays feed) down to
// the next few upcoming occurrences instead of a full agenda. A
// recurring birthday event is expanded server-side (internal/plugins/
// data/icsfeed's rrule handling) into concrete future occurrences
// already carrying the correct upcoming year, so no birthday-specific
// "ignore the year" math is needed here.
function UpcomingBirthdaysComponent({ data, config, theme }: WidgetProps<EventRow>) {
  const cfg = config as Config;
  const count = Math.max(1, cfg.count ?? 5);
  const now = new Date();

  const upcoming = data
    .map((e) => ({ event: e, startDate: parseSQLDateTime(e.start) }))
    .filter(({ startDate }) => daysUntil(now, startDate) >= 0)
    .sort((a, b) => a.startDate.getTime() - b.startDate.getTime())
    .slice(0, count);

  const style = cardStyle(theme);

  return (
    <div className="birthdays-widget mullet-card" style={style}>
      {upcoming.length === 0 ? (
        <div className="birthdays-empty">No upcoming birthdays</div>
      ) : (
        upcoming.map(({ event, startDate }) => (
          <div className="birthday-row" key={event.id}>
            <span className="birthday-name">{event.title}</span>
            <span className="birthday-date">
              {startDate.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}
            </span>
            <span className="birthday-relative" style={{ color: theme.accentColor }}>
              {relativeDayLabel(daysUntil(now, startDate))}
            </span>
          </div>
        ))
      )}
    </div>
  );
}

export const birthdaysPlugin: UIPlugin<EventRow> = {
  id: 'mullet-birthdays',
  name: 'Upcoming Birthdays',
  dataShape: 'events',
  defaultSize: { w: 3, h: 4 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 5, h: 8 },
  configSchema: {
    count: {
      type: 'number',
      label: 'Number to show',
      default: 5,
      helpText: 'How many upcoming birthdays to list.',
    },
  },
  component: UpcomingBirthdaysComponent,
};

import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import type { EventRow } from '../calendar-agenda/CalendarAgendaWidget';
import './MealPlanWidget.css';

interface Config {
  keyword?: string;
  days?: number;
}

function parseSQLDateTime(raw: string): Date {
  // See calendar-agenda's identical helper: a real instant, grouped
  // into a calendar day via the viewer's local timezone, not UTC's.
  return new Date(raw.replace(' ', 'T') + 'Z');
}

function localDayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// stripKeyword removes a leading "keyword:" or "keyword -" prefix some
// meal-planning conventions use (e.g. an event titled "Dinner: Tacos"
// filtered on keyword "Dinner" shows just "Tacos") -- falls back to the
// full title unchanged if the event doesn't happen to start that way.
function stripKeyword(title: string, keyword: string): string {
  const re = new RegExp(`^\\s*${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*[:\\-]\\s*`, 'i');
  return title.replace(re, '') || title;
}

function MealPlanComponent({ data, config, size, theme }: WidgetProps<EventRow>) {
  const cfg = config as Config;
  const keyword = (cfg.keyword ?? 'meal').trim();
  const requestedDays = cfg.days ?? 7;
  // A narrow card can't fit 7 day-columns legibly -- same reasoning as
  // weather-forecast's own width-based day cap.
  const maxDaysForWidth = size.w <= 5 ? 3 : size.w <= 8 ? 5 : 7;
  const days = Math.max(1, Math.min(requestedDays, maxDaysForWidth));

  const matches = keyword
    ? data.filter((e) => e.title.toLowerCase().includes(keyword.toLowerCase()) || (e.description ?? '').toLowerCase().includes(keyword.toLowerCase()))
    : data;

  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const dayList: { key: string; date: Date }[] = [];
  for (let i = 0; i < days; i++) {
    const d = new Date(today);
    d.setDate(d.getDate() + i);
    dayList.push({ key: localDayKey(d), date: d });
  }

  const byDay = new Map<string, EventRow[]>(dayList.map(({ key }) => [key, []]));
  for (const e of matches) {
    const key = localDayKey(parseSQLDateTime(e.start));
    if (byDay.has(key)) byDay.get(key)!.push(e);
  }

  const style = cardStyle(theme);

  return (
    <div className="meal-plan-widget mullet-card" style={{ ...style, gridTemplateColumns: `repeat(${days}, 1fr)` }}>
      {dayList.map(({ key, date }) => {
        const meals = byDay.get(key)!;
        return (
          <div className="mp-day" key={key}>
            <div className="mp-day-header">{date.toLocaleDateString(undefined, { weekday: 'short' })}</div>
            {meals.length === 0 ? (
              <div className="mp-empty">—</div>
            ) : (
              meals.map((m) => (
                <div className="mp-meal" key={m.id}>
                  {stripKeyword(m.title, keyword)}
                </div>
              ))
            )}
          </div>
        );
      })}
    </div>
  );
}

export const mealPlanPlugin: UIPlugin<EventRow> = {
  id: 'mullet-meal-plan',
  name: 'Meal Plan',
  dataShape: 'events',
  defaultSize: { w: 10, h: 4 },
  minSize: { w: 4, h: 3 },
  maxSize: { w: 16, h: 6 },
  configSchema: {
    keyword: {
      type: 'text', label: 'Keyword filter', default: 'meal',
      helpText: 'Only events whose title or description contains this word show up (e.g. "Dinner").',
    },
    days: { type: 'number', label: 'Days to show', default: 7 },
  },
  component: MealPlanComponent,
};

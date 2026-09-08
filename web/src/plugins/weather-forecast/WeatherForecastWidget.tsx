import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { conditionGlyph } from '../shared/conditionIcons';
import './WeatherForecastWidget.css';

// One row from GET /api/data/weather_forecast -- field names match the
// shape_weather_forecast columns verbatim.
export interface WeatherForecastRow {
  id: string;
  date: string;
  high: number;
  low: number;
  condition: string;
  icon: string;
  precip_chance: number | null;
  fetched_at: string;
}

interface Config {
  unit?: string;
  days?: number;
}

function round(n: number): string {
  return String(Math.round(n));
}

function dayLabel(dateStr: string): string {
  const d = new Date(`${dateStr}T00:00:00`);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString(undefined, { weekday: 'short' });
}

function WeatherForecastComponent({ data, config, size, theme }: WidgetProps<WeatherForecastRow>) {
  const cfg = config as Config;
  const unit = cfg.unit ?? '°F';
  const requestedDays = cfg.days ?? 5;
  // A narrow cell can't fit 5 day-columns legibly -- show fewer rather
  // than squeezing or scrolling. ~90 grid-px per day column is a rough
  // fit; 1 grid unit here stands for the screen's own column width,
  // which the widget doesn't know in pixels, so this just tracks grid
  // units directly (consistent with weather-current's own compacting).
  const maxDaysForWidth = size.w <= 4 ? 2 : size.w <= 8 ? 3 : size.w <= 12 ? 4 : 5;
  const days = [...data].sort((a, b) => a.date.localeCompare(b.date)).slice(0, Math.min(requestedDays, maxDaysForWidth, data.length));

  const style = cardStyle(theme);

  if (days.length === 0) {
    return (
      <div className="weather-forecast-widget wf-empty" style={style}>
        No forecast data yet
      </div>
    );
  }

  return (
    <div className="weather-forecast-widget" style={style}>
      {days.map((d) => (
        <div className="wf-day" key={d.id}>
          <div className="wf-label">{dayLabel(d.date)}</div>
          <div className="wf-glyph">{conditionGlyph(d.condition)}</div>
          <div className="wf-hi-lo">
            <span className="wf-high" style={{ color: theme.accentColor }}>
              {round(d.high)}
              {unit}
            </span>
            <span className="wf-low">
              {round(d.low)}
              {unit}
            </span>
          </div>
          {d.precip_chance != null && d.precip_chance > 0 && <div className="wf-precip">☔ {d.precip_chance}%</div>}
        </div>
      ))}
    </div>
  );
}

export const weatherForecastPlugin: UIPlugin<WeatherForecastRow> = {
  id: 'mullet-weather-forecast',
  name: 'Weather (Forecast)',
  dataShape: 'weather_forecast',
  defaultSize: { w: 8, h: 3 },
  minSize: { w: 4, h: 2 },
  maxSize: { w: 16, h: 4 },
  configSchema: {
    unit: {
      type: 'select',
      label: 'Temperature unit',
      default: '°F',
      options: [
        { value: '°F', label: 'Fahrenheit' },
        { value: '°C', label: 'Celsius' },
      ],
    },
    days: { type: 'number', label: 'Days to show', default: 5 },
  },
  component: WeatherForecastComponent,
};

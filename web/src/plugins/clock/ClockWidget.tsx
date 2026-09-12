import { useEffect, useState } from 'react';
import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './ClockWidget.css';

interface Config {
  displayMode?: 'digital' | 'analog';
  format?: '12h' | '24h';
  showSeconds?: boolean;
  showDate?: boolean;
  dateFormat?: 'short' | 'long' | 'numeric' | 'custom';
  customDateFormat?: string;
}

function formatTime(d: Date, format: '12h' | '24h', showSeconds: boolean): string {
  return d.toLocaleTimeString(undefined, {
    hour: 'numeric',
    minute: '2-digit',
    second: showSeconds ? '2-digit' : undefined,
    hour12: format !== '24h',
  });
}

// Moment.js-style tokens, for the one date shape the locale-driven
// Intl options below can't produce: a fixed field order (e.g.
// "MM/DD/YY") regardless of the viewer's locale. Longest tokens are
// listed first in the regex so e.g. "MMMM" isn't consumed as "MM"+"MM".
const CUSTOM_DATE_TOKENS = /YYYY|YY|MMMM|MMM|MM|M|dddd|ddd|DD|D/g;

function formatCustomDate(d: Date, pattern: string): string {
  const pad2 = (n: number) => String(n).padStart(2, '0');
  const tokens: Record<string, string> = {
    YYYY: String(d.getFullYear()),
    YY: String(d.getFullYear()).slice(-2),
    MMMM: d.toLocaleDateString(undefined, { month: 'long' }),
    MMM: d.toLocaleDateString(undefined, { month: 'short' }),
    MM: pad2(d.getMonth() + 1),
    M: String(d.getMonth() + 1),
    dddd: d.toLocaleDateString(undefined, { weekday: 'long' }),
    ddd: d.toLocaleDateString(undefined, { weekday: 'short' }),
    DD: pad2(d.getDate()),
    D: String(d.getDate()),
  };
  return pattern.replace(CUSTOM_DATE_TOKENS, (match) => tokens[match]);
}

function formatDate(d: Date, dateFormat: 'short' | 'long' | 'numeric' | 'custom', customDateFormat: string): string {
  switch (dateFormat) {
    case 'long':
      return d.toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' });
    case 'numeric':
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'numeric', day: 'numeric' });
    case 'custom':
      return formatCustomDate(d, customDateFormat);
    case 'short':
    default:
      return d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
  }
}

// Hands are drawn pointing straight up from the face's center, then
// rotated clockwise by their angle -- SVG's rotate() already turns
// clockwise in the default (y-down) coordinate system, matching how
// clock hands actually move, so no sign flipping is needed.
function AnalogClockFace({ d, showSeconds, textColor, accentColor }: { d: Date; showSeconds: boolean; textColor: string; accentColor: string }) {
  const hours = d.getHours() % 12;
  const minutes = d.getMinutes();
  const seconds = d.getSeconds();
  const hourAngle = (hours + minutes / 60) * 30;
  const minuteAngle = (minutes + seconds / 60) * 6;
  const secondAngle = seconds * 6;

  return (
    <svg className="clock-analog-face" viewBox="0 0 100 100" role="img" aria-label={d.toLocaleTimeString()}>
      <circle cx="50" cy="50" r="47" fill="none" stroke={textColor} strokeOpacity={0.25} strokeWidth={2} />
      {Array.from({ length: 12 }, (_, i) => {
        const angle = i * 30;
        return (
          <line
            key={i}
            x1="50" y1="6" x2="50" y2="12"
            stroke={textColor}
            strokeOpacity={0.4}
            strokeWidth={i % 3 === 0 ? 2.5 : 1.5}
            transform={`rotate(${angle} 50 50)`}
          />
        );
      })}
      <line x1="50" y1="50" x2="50" y2="26" stroke={textColor} strokeWidth={4} strokeLinecap="round" transform={`rotate(${hourAngle} 50 50)`} />
      <line x1="50" y1="50" x2="50" y2="14" stroke={textColor} strokeWidth={3} strokeLinecap="round" transform={`rotate(${minuteAngle} 50 50)`} />
      {showSeconds && (
        <line x1="50" y1="56" x2="50" y2="10" stroke={accentColor} strokeWidth={1.5} strokeLinecap="round" transform={`rotate(${secondAngle} 50 50)`} />
      )}
      <circle cx="50" cy="50" r="3" fill={accentColor} />
    </svg>
  );
}

// ClockWidget needs no data source at all -- dataShape: '' means the
// display never even fetches for it (see useShapeData.ts).
function ClockComponent({ config, size, theme }: WidgetProps<unknown>) {
  const cfg = config as Config;
  const displayMode = cfg.displayMode ?? 'digital';
  const format = cfg.format ?? '12h';
  const showSeconds = cfg.showSeconds === true;
  const showDate = cfg.showDate !== false;
  const dateFormat = cfg.dateFormat ?? 'long';
  const customDateFormat = cfg.customDateFormat || 'MM/DD/YY';
  const compact = size.h <= 2 || size.w <= 3;

  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const interval = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  const style = cardStyle(theme);

  return (
    <div className={`clock-widget mullet-card${compact ? ' clock-compact' : ''}`} style={style}>
      {displayMode === 'analog' ? (
        <AnalogClockFace d={now} showSeconds={showSeconds} textColor={theme.textColor} accentColor={theme.accentColor} />
      ) : (
        <span className="clock-time">{formatTime(now, format, showSeconds)}</span>
      )}
      {showDate && !compact && <span className="clock-date">{formatDate(now, dateFormat, customDateFormat)}</span>}
    </div>
  );
}

export const clockPlugin: UIPlugin<unknown> = {
  id: 'mullet-clock',
  name: 'Clock',
  dataShape: '',
  defaultSize: { w: 3, h: 2 },
  minSize: { w: 2, h: 1 },
  maxSize: { w: 6, h: 4 },
  configSchema: {
    displayMode: {
      type: 'select', label: 'Display', default: 'digital',
      options: [
        { value: 'digital', label: 'Digital' },
        { value: 'analog', label: 'Analog' },
      ],
    },
    format: {
      type: 'select', label: 'Time format', default: '12h',
      options: [
        { value: '12h', label: '12-hour' },
        { value: '24h', label: '24-hour (military)' },
      ],
      helpText: 'Digital display only',
    },
    showSeconds: { type: 'toggle', label: 'Show seconds', default: false },
    showDate: { type: 'toggle', label: 'Show date', default: true },
    dateFormat: {
      type: 'select', label: 'Date format', default: 'long',
      options: [
        { value: 'short', label: 'Mon, Jan 5' },
        { value: 'long', label: 'January 5, 2026' },
        { value: 'numeric', label: '1/5/2026' },
        { value: 'custom', label: 'Custom...' },
      ],
    },
    customDateFormat: {
      type: 'text', label: 'Custom date format', default: 'MM/DD/YY',
      helpText: 'Used when Date format is "Custom...". Tokens: YYYY, YY, MMMM, MMM, MM, M, dddd, ddd, DD, D',
    },
  },
  component: ClockComponent,
};

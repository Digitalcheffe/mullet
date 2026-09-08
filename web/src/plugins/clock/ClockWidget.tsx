import { useEffect, useState } from 'react';
import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './ClockWidget.css';

interface Config {
  format?: '12h' | '24h';
  showDate?: boolean;
}

function formatTime(d: Date, format: '12h' | '24h'): string {
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit', hour12: format !== '24h' });
}

function formatDate(d: Date): string {
  return d.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
}

// ClockWidget needs no data source at all -- dataShape: '' means the
// display never even fetches for it (see useShapeData.ts).
function ClockComponent({ config, size, theme }: WidgetProps<unknown>) {
  const cfg = config as Config;
  const format = cfg.format ?? '12h';
  const showDate = cfg.showDate !== false;
  const compact = size.h <= 2 || size.w <= 3;

  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const interval = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  const style = cardStyle(theme);

  return (
    <div className={`clock-widget${compact ? ' clock-compact' : ''}`} style={style}>
      <span className="clock-time">{formatTime(now, format)}</span>
      {showDate && !compact && <span className="clock-date">{formatDate(now)}</span>}
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
    format: {
      type: 'select', label: 'Time format', default: '12h',
      options: [
        { value: '12h', label: '12-hour' },
        { value: '24h', label: '24-hour' },
      ],
    },
    showDate: { type: 'toggle', label: 'Show date', default: true },
  },
  component: ClockComponent,
};

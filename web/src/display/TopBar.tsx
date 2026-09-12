import { useEffect, useState } from 'react';
import type { ThemeTokens } from '../shared/themes/tokens';
import { ConditionIcon, isNightTime } from '../plugins/shared/conditionIcons';
import { useShapeData } from './useShapeData';
import './TopBar.css';

interface Props {
  displayName: string;
  theme: ThemeTokens;
}

function formatTime(d: Date): string {
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function formatDate(d: Date): string {
  return d.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
}

// TopBar shows a live clock plus, when any weather_current data exists
// anywhere on the server, a compact condition/temp summary -- it isn't
// tied to a specific screen or card, so it works the same regardless of
// which screen is currently rotated into view. There's no per-display
// "which weather instance is the summary one" setting yet, so with more
// than one weather source configured this just shows whichever the API
// returns first; a real disambiguation control is future work.
export default function TopBar({ displayName, theme }: Props) {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const interval = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  const weather = useShapeData('weather_current', null)[0];

  return (
    <div className="display-top-bar" style={{ color: theme.textColor, fontFamily: theme.fontFamily }}>
      <div className="tb-display-name" style={{ fontFamily: theme.headingFontFamily }}>
        {displayName}
      </div>
      <div className="tb-clock">
        <span className="tb-time">{formatTime(now)}</span>
        <span className="tb-date">{formatDate(now)}</span>
      </div>
      {weather && (
        <div className="tb-weather" style={{ color: theme.accentColor }}>
          <ConditionIcon
            condition={String(weather.condition ?? '')}
            isNight={isNightTime(weather.sunrise as string | null, weather.sunset as string | null)}
            width="1em"
            height="1em"
          />
          <span>{Math.round(Number(weather.temp))}°</span>
        </div>
      )}
    </div>
  );
}

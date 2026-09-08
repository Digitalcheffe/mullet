import type { ThemeTokens } from '../shared/themes/tokens';
import { useShapeData } from './useShapeData';
import './BottomBar.css';

interface Props {
  theme: ThemeTokens;
  screenCount: number;
  activeScreen: number;
  reconnecting: boolean;
}

// BottomBar shows a weather alert when the configured weather source
// has one (issue #31), connection status, and, when a display rotates
// through more than one screen, which one is currently up. Reads
// weather_current itself, the same way TopBar independently reads it
// for its own compact summary -- neither is tied to a specific screen,
// so both work the same regardless of which screen is rotated into
// view. `alert` is nil for every built weather plugin today (see
// migration 012_weather_alert.sql's comment on why); this just renders
// it when a future plugin populates it.
export default function BottomBar({ theme, screenCount, activeScreen, reconnecting }: Props) {
  const weather = useShapeData('weather_current', null)[0];
  const alert = weather?.alert ? String(weather.alert) : null;

  return (
    <div className="display-bottom-bar" style={{ color: theme.textColor, fontFamily: theme.fontFamily }}>
      {alert && (
        <div className="bb-alert">
          <span className="bb-alert-dot" />
          {alert}
        </div>
      )}
      <div className={`bb-status ${reconnecting ? 'bb-status-bad' : 'bb-status-ok'}`}>
        <span className="bb-status-dot" />
        {reconnecting ? 'Reconnecting…' : 'Connected'}
      </div>
      {screenCount > 1 && (
        <div className="bb-screen-dots">
          {Array.from({ length: screenCount }, (_, i) => (
            <span
              key={i}
              className="bb-dot"
              style={i === activeScreen ? { background: theme.accentColor } : undefined}
            />
          ))}
        </div>
      )}
    </div>
  );
}

import type { ThemeTokens } from '../shared/themes/tokens';
import './BottomBar.css';

interface Props {
  theme: ThemeTokens;
  screenCount: number;
  activeScreen: number;
  reconnecting: boolean;
}

// BottomBar shows connection status and, when a display rotates through
// more than one screen, which one is currently up -- the "alerts" half
// of the issue's bar description has no backing data source yet (no
// shape carries alert-style events), so this covers "status" only; see
// architecture.md's Divergence note.
export default function BottomBar({ theme, screenCount, activeScreen, reconnecting }: Props) {
  return (
    <div className="display-bottom-bar" style={{ color: theme.textColor, fontFamily: theme.fontFamily }}>
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

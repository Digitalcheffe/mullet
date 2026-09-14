import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { backgroundCSS } from '../shared/themes/tokens';
import BackgroundLayer from './BackgroundLayer';
import BottomBar from './BottomBar';
import ConnectOverlay from './ConnectOverlay';
import ScreenGrid from './ScreenGrid';
import TopBar from './TopBar';
import { useDisplayLayout, type DisplayLayout } from './useDisplayLayout';
import './DisplayApp.css';

// Night mode (issue #91) -- "HH:MM" in the display's own local time
// (this runs entirely client-side; a kiosk's browser clock/timezone is
// what actually matters, not the server's). start > end means the
// window wraps past midnight (e.g. "22:00"-"07:00" spans two calendar
// days), which is why this can't just compare start <= now <= end.
function parseHHMM(v: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(v);
  if (!m) return null;
  const h = Number(m[1]);
  const min = Number(m[2]);
  if (h > 23 || min > 59) return null;
  return h * 60 + min;
}

function isNightModeActive(layout: DisplayLayout, now: Date): boolean {
  if (!layout.night_mode_enabled || !layout.night_start || !layout.night_end) return false;
  const start = parseHHMM(layout.night_start);
  const end = parseHHMM(layout.night_end);
  if (start == null || end == null) return false;
  const minutesNow = now.getHours() * 60 + now.getMinutes();
  return start <= end ? minutesNow >= start && minutesNow < end : minutesNow >= start || minutesNow < end;
}

// Re-checked every 30s rather than once -- the window's start/end
// boundary needs to actually be crossed while the kiosk stays open for
// days at a time, not just evaluated at load.
const NIGHT_MODE_CHECK_MS = 30 * 1000;

// Display renderer entry point: full-screen grid engine, screen
// rotation, and top/bottom bars for one display (see architecture.md
// "Display Hierarchy").
export default function DisplayApp() {
  const { slug } = useParams<{ slug: string }>();
  const { layout, error, notFound } = useDisplayLayout(slug);
  const [activeScreen, setActiveScreen] = useState(0);
  const [nightModeActive, setNightModeActive] = useState(false);

  useEffect(() => {
    if (!layout || layout.screens.length <= 1) return;
    const seconds = layout.rotation_seconds > 0 ? layout.rotation_seconds : 30;
    const interval = setInterval(() => {
      setActiveScreen((i) => (i + 1) % layout.screens.length);
    }, seconds * 1000);
    return () => clearInterval(interval);
  }, [layout]);

  useEffect(() => {
    if (!layout) return;
    const check = () => setNightModeActive(isNightModeActive(layout, new Date()));
    check();
    const interval = setInterval(check, NIGHT_MODE_CHECK_MS);
    return () => clearInterval(interval);
  }, [layout]);

  if (!layout) {
    return <ConnectOverlay mode={notFound ? 'not-found' : 'connecting'} slug={slug ?? ''} />;
  }

  // Clamp rather than reset-via-effect: if an admin edit shrinks the
  // screen list mid-rotation, this settles on the last screen
  // immediately instead of flashing "no screens" for one tick while a
  // separate effect catches up.
  const screen = layout.screens[Math.min(activeScreen, layout.screens.length - 1)];
  // An image background renders via BackgroundLayer instead (issue
  // #92's Ken Burns motion/slideshow/scrim need actual DOM layers, not
  // a single CSS `background` value) -- solid/gradient still just set
  // it directly, unchanged from before that existed.
  const isImageBackground = layout.theme.background.type === 'image';
  const background = isImageBackground ? undefined : backgroundCSS(layout.theme.background);
  const style = nightModeActive ? { background, filter: `brightness(${layout.night_brightness})` } : { background };

  return (
    <div className="display-app" style={style}>
      {isImageBackground && <BackgroundLayer background={layout.theme.background} />}
      <div className="display-app-content">
        {layout.show_top_bar && <TopBar displayName={layout.name} theme={layout.theme} />}
        {screen ? (
          <div className="display-app-screen">
            {/* Keyed by screen id (issue #86) -- forces a full remount on
                rotation so the fade-in/card-entrance CSS animations in
                ScreenGrid.css replay each time, instead of only once ever. */}
            <ScreenGrid key={screen.id} screen={screen} theme={layout.theme} />
          </div>
        ) : (
          <div className="display-app-empty" style={{ color: layout.theme.textColor }}>
            No screens configured for this display yet.
          </div>
        )}
        {layout.show_bottom_bar && (
          <BottomBar
            theme={layout.theme}
            screenCount={layout.screens.length}
            activeScreen={activeScreen}
            reconnecting={error !== null}
          />
        )}
        {error && <ConnectOverlay mode="reconnecting" slug={slug ?? ''} />}
      </div>
    </div>
  );
}

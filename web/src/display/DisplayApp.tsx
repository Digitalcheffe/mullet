import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { backgroundCSS } from '../shared/themes/tokens';
import BottomBar from './BottomBar';
import ConnectOverlay from './ConnectOverlay';
import ScreenGrid from './ScreenGrid';
import TopBar from './TopBar';
import { useDisplayLayout } from './useDisplayLayout';
import './DisplayApp.css';

// Display renderer entry point: full-screen grid engine, screen
// rotation, and top/bottom bars for one display (see architecture.md
// "Display Hierarchy").
export default function DisplayApp() {
  const { slug } = useParams<{ slug: string }>();
  const { layout, error, notFound } = useDisplayLayout(slug);
  const [activeScreen, setActiveScreen] = useState(0);

  useEffect(() => {
    if (!layout || layout.screens.length <= 1) return;
    const seconds = layout.rotation_seconds > 0 ? layout.rotation_seconds : 30;
    const interval = setInterval(() => {
      setActiveScreen((i) => (i + 1) % layout.screens.length);
    }, seconds * 1000);
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
  const background = backgroundCSS(layout.theme.background);

  return (
    <div className="display-app" style={{ background }}>
      {layout.show_top_bar && <TopBar displayName={layout.name} theme={layout.theme} />}
      {screen ? (
        <div className="display-app-screen">
          <ScreenGrid screen={screen} theme={layout.theme} />
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
  );
}

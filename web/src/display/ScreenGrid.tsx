import { useEffect } from 'react';
import type { ThemeTokens } from '../shared/themes/tokens';
import { ensureFontLoaded } from '../shared/themes/fontOptions';
import DisplayCard from './DisplayCard';
import type { ScreenLayout } from './useDisplayLayout';
import './ScreenGrid.css';

interface Props {
  screen: ScreenLayout;
  theme: ThemeTokens;
}

// ScreenGrid lays out one screen's cards with plain CSS Grid -- a
// read-only render of the same x/y/w/h coordinates the Designer's
// react-grid-layout uses for editing, so a card that fits without
// overlap in the Designer fits identically here (grid lines are
// 1-indexed, hence the `+1`s in DisplayCard).
//
// Rows use `1fr` sizing (via an explicit `grid-template-rows`, one `fr`
// per row actually used), the same way columns already do -- not the
// screen's stored `row_height` in raw pixels (issue #31). A fixed pixel
// row height was tuned for whatever resolution the admin happened to
// be designing in; opened on a different aspect ratio (a portrait
// tablet, an ultrawide) it either overflows past the bottom of the
// viewport (clipped, since the display route hides overflow) or leaves
// dead space, depending on which way the mismatch goes. `1fr` rows
// scale the whole grid to fill exactly the container's actual height
// on any screen shape, with no overflow and no gap, matching how the
// width axis already behaves.
function rowCount(screen: ScreenLayout): number {
  let max = 1;
  for (const card of screen.cards) {
    max = Math.max(max, card.y + card.h);
  }
  return max;
}

export default function ScreenGrid({ screen, theme }: Props) {
  // A screen's own font override (issue #87) -- same shallow-spread
  // merge DisplayCard already does one level down for a card's own
  // theme_override, just applied here first so every card on this
  // screen inherits it unless a card overrides its own font too (cards
  // can't today -- CARD_OVERRIDE_FIELDS excludes font -- but the merge
  // order still falls out correctly if that ever changes).
  const resolvedTheme: ThemeTokens = { ...theme, ...(screen.theme_override ?? {}) };

  // Loads whichever curated Google Font this screen's resolved theme
  // needs, the first time it's rendered -- a kiosk display never
  // otherwise touches the theme editor, so nothing else would trigger
  // the fetch.
  useEffect(() => {
    ensureFontLoaded(resolvedTheme.fontFamily);
    ensureFontLoaded(resolvedTheme.headingFontFamily);
  }, [resolvedTheme.fontFamily, resolvedTheme.headingFontFamily]);

  const style = {
    gridTemplateColumns: `repeat(${screen.columns}, 1fr)`,
    gridTemplateRows: `repeat(${rowCount(screen)}, 1fr)`,
    gap: `${screen.gap}px`,
  };

  return (
    <div className="screen-grid" style={style}>
      {screen.cards.map((card) => (
        <DisplayCard key={card.id} card={card} theme={resolvedTheme} />
      ))}
    </div>
  );
}

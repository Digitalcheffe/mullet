import type { ThemeTokens } from '../shared/themes/tokens';
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
export default function ScreenGrid({ screen, theme }: Props) {
  const style = {
    gridTemplateColumns: `repeat(${screen.columns}, 1fr)`,
    gridAutoRows: `${screen.row_height}px`,
    gap: `${screen.gap}px`,
  };

  return (
    <div className="screen-grid" style={style}>
      {screen.cards.map((card) => (
        <DisplayCard key={card.id} card={card} theme={theme} />
      ))}
    </div>
  );
}

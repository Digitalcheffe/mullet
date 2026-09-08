import type { CSSProperties } from 'react';
import type { ThemeTokens } from '../../shared/themes/tokens';

// The visual chrome every UI plugin renders for itself -- there's no
// separate wrapping "Card" component in the current design (see
// architecture.md "The Designer"), so each widget is responsible for
// its own themed background/border/blur/text styling.
export function cardStyle(theme: ThemeTokens): CSSProperties {
  return {
    background: theme.cardBackground,
    border: `1px solid ${theme.cardBorder}`,
    borderRadius: theme.borderRadius,
    backdropFilter: `blur(${theme.blur})`,
    opacity: theme.opacity,
    color: theme.textColor,
    fontFamily: theme.fontFamily,
    fontSize: theme.fontSize,
  };
}

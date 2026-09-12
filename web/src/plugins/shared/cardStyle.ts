import type { CSSProperties } from 'react';
import type { ThemeTokens } from '../../shared/themes/tokens';
import './cardStyle.css';

// The visual chrome every UI plugin renders for itself -- there's no
// separate wrapping "Card" component in the current design (see
// architecture.md "The Designer"), so each widget is responsible for
// its own themed background/border/blur/text styling. Every widget's
// root element must pair this style with the `mullet-card` class name
// (in addition to its own) for the background layer below to apply.
//
// `opacity` only affects the background/blur -- not the card's own
// text/icons -- by way of a `::before` pseudo-element (`.mullet-card`
// in cardStyle.css) carrying the background, border, and blur, fed by
// the --mullet-card-* custom properties below (the only way to reach a
// pseudo-element from an inline React style). Lowering opacity used to
// fade the whole card, text included, into illegibility -- see issue
// #75. `color`/`fontFamily`/`fontSize` stay as ordinary properties on
// the root itself, which the pseudo-element's opacity has no effect on.
export function cardStyle(theme: ThemeTokens): CSSProperties {
  // 'solid' is a flat info-block chrome (issue #90) -- no backdrop blur
  // regardless of the theme's own blur token, since blur is specifically
  // a glass-look concept; a soft shadow either way, but a stronger one
  // for glass, where it does more work carrying the card off the
  // background since there's no opaque fill contrast to lean on.
  const isSolid = theme.cardStyle === 'solid';
  return {
    borderRadius: theme.borderRadius,
    color: theme.textColor,
    fontFamily: theme.fontFamily,
    fontSize: theme.fontSize,
    '--mullet-card-background': theme.cardBackground,
    '--mullet-card-border': theme.cardBorder,
    '--mullet-card-blur': isSolid ? '0px' : theme.blur,
    '--mullet-card-opacity': theme.opacity,
    '--mullet-card-shadow': isSolid ? '0 1px 2px rgba(0, 0, 0, 0.12)' : '0 10px 30px rgba(0, 0, 0, 0.22)',
    '--mullet-card-highlight': isSolid ? 'transparent' : 'rgba(255, 255, 255, 0.16)',
    '--mullet-heading-font': theme.headingFontFamily || theme.fontFamily,
    '--mullet-font-sm': theme.fontSizeSmall,
    '--mullet-font-lg': theme.fontSizeLarge,
    '--mullet-success': theme.successColor,
    '--mullet-warning': theme.warningColor,
    '--mullet-error': theme.errorColor,
    '--mullet-info': theme.infoColor,
  } as CSSProperties;
}

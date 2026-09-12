// Theme token types and the built-in default, used by both the display
// renderer and the admin theme editor (see architecture.md "Theme Tables").

export interface ThemeTokens {
  background: { type: 'solid' | 'gradient' | 'image'; value: string };
  cardBackground: string;
  cardBorder: string;
  // 'glass' keeps the existing translucent-blur look (now with a drop
  // shadow + inner top-edge highlight); 'solid' is a flat info-block
  // alternative for themes that shouldn't be forced translucent.
  cardStyle: 'glass' | 'solid';
  textColor: string;
  accentColor: string;
  // Semantic colors -- previously hardcoded per-widget (ServerHealth and
  // HomeStatus each independently defined the same green/amber/red), now
  // theme-aware so a widget's status coloring follows the theme instead
  // of a fixed palette.
  successColor: string;
  warningColor: string;
  errorColor: string;
  infoColor: string;
  fontFamily: string;
  fontSize: string;
  // Optional heading typeface -- falls back to fontFamily when unset, so
  // a theme can give headings/hero numbers their own voice without every
  // theme needing to specify one.
  headingFontFamily: string;
  // A minimal 3-step size ramp (fontSize itself is the middle "body"
  // step) -- lets a widget mark a label vs. a hero number as distinct
  // roles instead of each widget inventing its own em multipliers off
  // the single base size.
  fontSizeSmall: string;
  fontSizeLarge: string;
  borderRadius: string;
  opacity: number;
  blur: string;
}

// backgroundCSS turns a theme's background token into a CSS `background`
// shorthand value -- solid and gradient values are already valid CSS on
// their own, but an image needs `url(...)` plus sizing/positioning.
export function backgroundCSS(background: ThemeTokens['background']): string {
  return background.type === 'image' ? `center/cover no-repeat url(${background.value})` : background.value;
}

export const defaultTheme: ThemeTokens = {
  background: { type: 'gradient', value: 'linear-gradient(160deg, #0b0f14 0%, #131a2b 60%, #1b2740 100%)' },
  cardBackground: 'rgba(255, 255, 255, 0.06)',
  cardBorder: 'rgba(255, 255, 255, 0.12)',
  cardStyle: 'glass',
  textColor: '#e6e9ee',
  accentColor: '#4f9dff',
  successColor: '#2ecc71',
  warningColor: '#f1c40f',
  errorColor: '#e74c3c',
  infoColor: '#4f9dff',
  fontFamily: 'system-ui, sans-serif',
  fontSize: '16px',
  headingFontFamily: 'system-ui, sans-serif',
  fontSizeSmall: '13px',
  fontSizeLarge: '28px',
  borderRadius: '12px',
  opacity: 1,
  blur: '16px',
};

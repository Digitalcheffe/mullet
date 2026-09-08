// Theme token types and the built-in default, used by both the display
// renderer and the admin theme editor (see architecture.md "Theme Tables").

export interface ThemeTokens {
  background: { type: 'solid' | 'gradient' | 'image'; value: string };
  cardBackground: string;
  cardBorder: string;
  textColor: string;
  accentColor: string;
  fontFamily: string;
  fontSize: string;
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
  background: { type: 'solid', value: '#0b0f14' },
  cardBackground: 'rgba(255, 255, 255, 0.06)',
  cardBorder: 'rgba(255, 255, 255, 0.12)',
  textColor: '#e6e9ee',
  accentColor: '#4f9dff',
  fontFamily: 'system-ui, sans-serif',
  fontSize: '16px',
  borderRadius: '12px',
  opacity: 1,
  blur: '16px',
};

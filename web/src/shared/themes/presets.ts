import { defaultTheme, type ThemeTokens } from './tokens';

// Bundled theme presets (issue #31) -- starting points offered in the
// admin's "New Theme" gallery, not a separate registry of their own;
// picking one just pre-fills the ordinary editable token form with its
// values. The same four (Dark Glass being `defaultTheme` itself) are
// seeded as real `themes` rows for a fresh install
// (internal/db/migrations/006_seed_default_theme.sql,
// 013_bundled_themes.sql) -- keep the two in sync by hand; nothing
// enforces it structurally.
export interface ThemePreset {
  name: string;
  tokens: ThemeTokens;
}

export const themePresets: ThemePreset[] = [
  { name: 'Dark Glass', tokens: defaultTheme },
  {
    name: 'Solid Dark',
    tokens: {
      background: { type: 'gradient', value: 'linear-gradient(160deg, #14181f 0%, #1c2230 100%)' },
      cardBackground: '#1f2530',
      cardBorder: '#2c3341',
      cardStyle: 'solid',
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
      blur: '0px',
    },
  },
  {
    name: 'Glass Light',
    tokens: {
      background: { type: 'gradient', value: 'linear-gradient(160deg, #f4f1ea 0%, #e8eef7 100%)' },
      cardBackground: 'rgba(255, 255, 255, 0.55)',
      cardBorder: 'rgba(0, 0, 0, 0.08)',
      cardStyle: 'glass',
      textColor: '#2a2320',
      accentColor: '#2f6fd6',
      successColor: '#3d9a6f',
      warningColor: '#c08a2e',
      errorColor: '#b5493c',
      infoColor: '#2f6fd6',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      headingFontFamily: 'system-ui, sans-serif',
      fontSizeSmall: '13px',
      fontSizeLarge: '28px',
      borderRadius: '12px',
      opacity: 1,
      blur: '16px',
    },
  },
  {
    name: 'Solid Light',
    tokens: {
      background: { type: 'gradient', value: 'linear-gradient(160deg, #f4f1ea 0%, #ece6da 100%)' },
      cardBackground: '#ffffff',
      cardBorder: '#e2ddd2',
      cardStyle: 'solid',
      textColor: '#2a2320',
      accentColor: '#2f6fd6',
      successColor: '#3d9a6f',
      warningColor: '#c08a2e',
      errorColor: '#b5493c',
      infoColor: '#2f6fd6',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      headingFontFamily: 'system-ui, sans-serif',
      fontSizeSmall: '13px',
      fontSizeLarge: '28px',
      borderRadius: '12px',
      opacity: 1,
      blur: '0px',
    },
  },
  {
    // Uses Mullet's own logo palette (navy/cyan/red-pink) -- previously
    // no bundled theme, or any display theme at all, used the brand's
    // actual colors (see issue #90's "brand color reconciliation" task).
    name: 'Mullet Brand',
    tokens: {
      background: { type: 'gradient', value: 'linear-gradient(160deg, #011D4C 0%, #050b1a 100%)' },
      cardBackground: 'rgba(255, 255, 255, 0.07)',
      cardBorder: 'rgba(1, 190, 252, 0.25)',
      cardStyle: 'glass',
      textColor: '#eef6ff',
      accentColor: '#01BEFC',
      successColor: '#2ecc71',
      warningColor: '#f1c40f',
      errorColor: '#FB4460',
      infoColor: '#01BEFC',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      headingFontFamily: 'system-ui, sans-serif',
      fontSizeSmall: '13px',
      fontSizeLarge: '28px',
      borderRadius: '14px',
      opacity: 1,
      blur: '20px',
    },
  },
];

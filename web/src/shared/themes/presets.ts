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
      background: { type: 'solid', value: '#14181f' },
      cardBackground: '#1f2530',
      cardBorder: '#2c3341',
      textColor: '#e6e9ee',
      accentColor: '#4f9dff',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      borderRadius: '12px',
      opacity: 1,
      blur: '0px',
    },
  },
  {
    name: 'Glass Light',
    tokens: {
      background: { type: 'solid', value: '#f4f1ea' },
      cardBackground: 'rgba(255, 255, 255, 0.55)',
      cardBorder: 'rgba(0, 0, 0, 0.08)',
      textColor: '#2a2320',
      accentColor: '#2f6fd6',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      borderRadius: '12px',
      opacity: 1,
      blur: '16px',
    },
  },
  {
    name: 'Solid Light',
    tokens: {
      background: { type: 'solid', value: '#f4f1ea' },
      cardBackground: '#ffffff',
      cardBorder: '#e2ddd2',
      textColor: '#2a2320',
      accentColor: '#2f6fd6',
      fontFamily: 'system-ui, sans-serif',
      fontSize: '16px',
      borderRadius: '12px',
      opacity: 1,
      blur: '0px',
    },
  },
];

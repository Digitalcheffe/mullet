import type { ThemeTokens } from './tokens';
import { defaultTheme } from './tokens';

const BACKGROUND_TYPES = ['solid', 'gradient', 'image'];
const CARD_STYLES = ['glass', 'solid'];

// The string-valued ThemeTokens fields, checked generically below --
// keep in sync with the ThemeTokens interface itself if it ever gains
// or loses a field (there's no way to derive this list from the type
// at runtime).
const STRING_FIELDS: (keyof ThemeTokens)[] = [
  'cardBackground',
  'cardBorder',
  'textColor',
  'accentColor',
  'fontFamily',
  'fontSize',
  'borderRadius',
  'blur',
];

// Fields added after the original schema (issue #90) -- backfilled from
// defaultTheme when missing/invalid instead of failing validation, so a
// theme JSON exported before these existed can still be re-imported.
// The fields above stay strictly required: an import from this app's own
// export always has them, and silently backfilling an original field
// would risk applying a theme the admin never actually saw.
const BACKFILLED_STRING_FIELDS: (keyof ThemeTokens)[] = [
  'headingFontFamily',
  'fontSizeSmall',
  'fontSizeLarge',
  'successColor',
  'warningColor',
  'errorColor',
  'infoColor',
];

export type ValidateTokensResult = { ok: true; tokens: ThemeTokens } | { ok: false; error: string };

// validateThemeTokens checks a JSON value against the exact ThemeTokens
// shape before it's ever applied to the editor -- deliberately strict on
// the original schema (every field required, exact types) rather than
// merging with defaultTheme for anything missing, since silently
// backfilling a partial/malformed import would apply a theme the admin
// never actually saw or approved. The exception is BACKFILLED_STRING_FIELDS
// (added in issue #90): those get defaulted rather than rejected, purely
// so a theme exported before they existed can still be re-imported.
// Used by the theme editor's Import flow; export never needs this since
// it only ever writes tokens this same shape already produced.
export function validateThemeTokens(input: unknown): ValidateTokensResult {
  if (typeof input !== 'object' || input === null || Array.isArray(input)) {
    return { ok: false, error: 'Expected a JSON object with theme token fields.' };
  }
  const obj = input as Record<string, unknown>;

  const background = obj.background;
  if (typeof background !== 'object' || background === null || Array.isArray(background)) {
    return { ok: false, error: '"background" is missing or not an object.' };
  }
  const bg = background as Record<string, unknown>;
  if (typeof bg.type !== 'string' || !BACKGROUND_TYPES.includes(bg.type)) {
    return { ok: false, error: `"background.type" must be one of: ${BACKGROUND_TYPES.join(', ')}.` };
  }
  if (typeof bg.value !== 'string' || bg.value === '') {
    return { ok: false, error: '"background.value" must be a non-empty string.' };
  }

  for (const field of STRING_FIELDS) {
    if (typeof obj[field] !== 'string' || obj[field] === '') {
      return { ok: false, error: `"${field}" must be a non-empty string.` };
    }
  }

  const cardStyleRaw = obj.cardStyle;
  const cardStyle =
    typeof cardStyleRaw === 'string' && CARD_STYLES.includes(cardStyleRaw)
      ? (cardStyleRaw as ThemeTokens['cardStyle'])
      : defaultTheme.cardStyle;

  const backfilled: Record<string, string> = {};
  for (const field of BACKFILLED_STRING_FIELDS) {
    const raw = obj[field];
    backfilled[field] = typeof raw === 'string' && raw !== '' ? raw : (defaultTheme[field] as string);
  }

  if (typeof obj.opacity !== 'number' || Number.isNaN(obj.opacity) || obj.opacity < 0 || obj.opacity > 1) {
    return { ok: false, error: '"opacity" must be a number between 0 and 1.' };
  }

  return {
    ok: true,
    tokens: {
      background: { type: bg.type as ThemeTokens['background']['type'], value: bg.value },
      cardBackground: obj.cardBackground as string,
      cardBorder: obj.cardBorder as string,
      cardStyle,
      textColor: obj.textColor as string,
      accentColor: obj.accentColor as string,
      successColor: backfilled.successColor,
      warningColor: backfilled.warningColor,
      errorColor: backfilled.errorColor,
      infoColor: backfilled.infoColor,
      fontFamily: obj.fontFamily as string,
      fontSize: obj.fontSize as string,
      headingFontFamily: backfilled.headingFontFamily,
      fontSizeSmall: backfilled.fontSizeSmall,
      fontSizeLarge: backfilled.fontSizeLarge,
      borderRadius: obj.borderRadius as string,
      opacity: obj.opacity as number,
      blur: obj.blur as string,
    },
  };
}

// Curated font choices for the theme editor's Font family / Heading font
// family selects, and the screen-level font override (issue #87).
// Deliberately a fixed list rather than a free-text field: a typo'd or
// unavailable font name used to fail the same silent way -- the display
// just fell back to whatever system font the device had.
export interface FontOption {
  label: string;
  value: string;
  // Google Fonts css2 family query segment (e.g. "Poppins:wght@400;600;700")
  // for a font that needs to be fetched at runtime -- omitted for system
  // fonts (System UI, Georgia), which every device already has.
  googleFont?: string;
}

export const FONT_OPTIONS: FontOption[] = [
  { label: 'System UI', value: 'system-ui, sans-serif' },
  { label: 'Georgia (Serif)', value: 'Georgia, serif' },
  { label: 'Manrope', value: "'Manrope', sans-serif", googleFont: 'Manrope:wght@400;500;600;700;800' },
  { label: 'Inter', value: "'Inter', sans-serif", googleFont: 'Inter:wght@400;600;700' },
  { label: 'Space Grotesk', value: "'Space Grotesk', sans-serif", googleFont: 'Space+Grotesk:wght@500;600;700' },
  { label: 'Poppins', value: "'Poppins', sans-serif", googleFont: 'Poppins:wght@400;600;700' },
  { label: 'Playfair Display (Serif)', value: "'Playfair Display', serif", googleFont: 'Playfair+Display:wght@600;700' },
  { label: 'JetBrains Mono', value: "'JetBrains Mono', monospace", googleFont: 'JetBrains+Mono:wght@500;700' },
];

// Manrope and Space Grotesk are already loaded statically in index.html
// (the admin UI's own chrome uses them unconditionally, not just when a
// theme picks them) -- pre-seeding these means ensureFontLoaded never
// injects a redundant second <link> for either.
const loadedGoogleFonts = new Set<string>(['Manrope:wght@400;500;600;700;800', 'Space+Grotesk:wght@500;600;700']);

// Dynamically injects the Google Fonts <link> for whichever curated font
// a theme's fontFamily/headingFontFamily actually resolves to, the first
// time it's needed -- rather than statically loading every curated font
// up front regardless of use, which doesn't scale as the curated list
// grows and wastes bandwidth on a constrained kiosk device that only
// ever renders one or two fonts. A no-op for system fonts or an unknown
// value (e.g. a hand-edited JSON import using a font outside the
// curated list -- there's nothing to fetch for that either way).
export function ensureFontLoaded(fontFamilyValue: string | undefined): void {
  const option = FONT_OPTIONS.find((f) => f.value === fontFamilyValue);
  if (!option?.googleFont || loadedGoogleFonts.has(option.googleFont)) return;
  loadedGoogleFonts.add(option.googleFont);
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = `https://fonts.googleapis.com/css2?family=${option.googleFont}&display=swap`;
  document.head.appendChild(link);
}

// Curated font choices for the theme editor's Font family / Heading font
// family selects. Every non-system entry here must be loaded in
// index.html's Google Fonts <link> -- picking one that isn't would
// silently fall back to the browser default with no visual feedback.
// Deliberately a fixed list rather than a free-text field: a typo'd or
// unavailable font name used to fail the same silent way.
export interface FontOption {
  label: string;
  value: string;
}

export const FONT_OPTIONS: FontOption[] = [
  { label: 'System UI', value: 'system-ui, sans-serif' },
  { label: 'Manrope', value: "'Manrope', sans-serif" },
  { label: 'Inter', value: "'Inter', sans-serif" },
  { label: 'Space Grotesk', value: "'Space Grotesk', sans-serif" },
  { label: 'Poppins', value: "'Poppins', sans-serif" },
  { label: 'Playfair Display (Serif)', value: "'Playfair Display', serif" },
  { label: 'Georgia (Serif)', value: 'Georgia, serif' },
  { label: 'JetBrains Mono', value: "'JetBrains Mono', monospace" },
];

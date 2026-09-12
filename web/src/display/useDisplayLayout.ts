import { useEffect, useState } from 'react';
import { defaultTheme, type ThemeTokens } from '../shared/themes/tokens';

export interface CardLayout {
  id: number;
  ui_plugin_id: string;
  data_plugin_instance_id: number | null;
  x: number;
  y: number;
  w: number;
  h: number;
  config: Record<string, unknown>;
  theme_override?: Partial<ThemeTokens>;
}

export interface ScreenLayout {
  id: number;
  name: string;
  position: number;
  columns: number;
  row_height: number;
  gap: number;
  cards: CardLayout[];
  // A screen-level font override (issue #87), mirroring a card's own
  // theme_override -- merged over the display's theme in ScreenGrid,
  // same shallow-spread pattern DisplayCard already uses for cards.
  theme_override?: Partial<ThemeTokens>;
}

export interface DisplayLayout {
  id: number;
  name: string;
  slug: string;
  rotation_seconds: number;
  show_top_bar: boolean;
  show_bottom_bar: boolean;
  theme: ThemeTokens;
  screens: ScreenLayout[];
}

interface RawLayout extends Omit<DisplayLayout, 'theme'> {
  theme?: Partial<ThemeTokens>;
}

// LAYOUT_POLL_MS re-fetches the display's own screens/cards/theme
// periodically so admin changes (a moved card, a new screen) show up on
// a running kiosk without a manual reload -- separate from and much
// slower than each card's own data refresh (useShapeData), since a
// layout edit is rare compared to a data plugin's fetch interval.
const LAYOUT_POLL_MS = 5 * 60 * 1000;

// useDisplayLayout fetches and polls GET /api/display/{slug}. `error`
// is set whenever the most recent fetch failed, whether or not a
// previous fetch already succeeded -- the caller distinguishes "never
// loaded" (layout === null) from "lost connection after loading fine"
// (layout set, error set) to decide between a blocking connect screen
// and a reconnecting overlay over the last-known layout.
export function useDisplayLayout(slug: string | undefined) {
  const [layout, setLayout] = useState<DisplayLayout | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    if (!slug) return;
    let cancelled = false;

    async function load() {
      try {
        const res = await fetch(`/api/display/${encodeURIComponent(slug!)}`);
        if (res.status === 404) {
          if (!cancelled) {
            setNotFound(true);
            setError('not found');
          }
          return;
        }
        if (!res.ok) throw new Error(`status ${res.status}`);
        const raw: RawLayout = await res.json();
        if (cancelled) return;
        setLayout({ ...raw, theme: { ...defaultTheme, ...raw.theme } });
        setError(null);
        setNotFound(false);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : 'unreachable');
      }
    }

    load();
    const interval = setInterval(load, LAYOUT_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [slug]);

  return { layout, error, notFound };
}

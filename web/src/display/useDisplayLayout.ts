import { useEffect, useState } from 'react';
import { defaultTheme, type ThemeTokens } from '../shared/themes/tokens';

export interface CardLayout {
  id: number;
  ui_plugin_id: string;
  data_plugin_instance_id: number | null;
  // Present only for a multi-source card (issue #97) -- see
  // UIPlugin.supportsMultiDataSource.
  data_plugin_instance_ids?: number[];
  x: number;
  y: number;
  w: number;
  h: number;
  config: Record<string, unknown>;
  theme_override?: Partial<ThemeTokens>;
  // Optional card header (issue #145) -- absent/empty header_text means
  // no header renders at all. Always renders in its own reserved strip
  // at the top of the card (issue #154) -- header_halign is the only
  // alignment control.
  header_text?: string;
  header_halign?: 'left' | 'center' | 'right';
  // Positions a widget's own content within its card (issue #149) --
  // absent/'center' on both axes matches every card's behavior before
  // this existed (the widget stretches to fill the card), since that's
  // the only value that doesn't shrink the widget to its natural size
  // (see DisplayCard's contentAlignStyle). Best suited to a simple,
  // naturally-sized widget (Clock, Countdown, Quote) -- aligning a
  // list-based widget (a calendar, a task list) shrinks it to fit its
  // content, which for an unbounded list means growing to show all of
  // it rather than the scrollable box it'd otherwise be.
  content_halign?: 'left' | 'center' | 'right';
  content_valign?: 'top' | 'center' | 'bottom';
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
  // Night mode (issue #91) -- night_start/night_end are "HH:MM" in the
  // display's own local time, wrapping past midnight when start > end
  // (e.g. "22:00"-"07:00"). Both absent alongside night_mode_enabled
  // false is every display's default.
  night_mode_enabled: boolean;
  night_start?: string;
  night_end?: string;
  night_brightness: number;
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

import { useEffect, useRef, useState } from 'react';

interface ShapeResponse {
  data: Record<string, unknown>[];
  source: string;
  last_updated?: string;
}

// DATA_POLL_MS matches the shortest refresh_seconds a data plugin
// instance is likely configured with (see the plugin instance form's
// default) -- polling faster wouldn't show anything new, since the
// shape table only changes when the scheduler re-runs the plugin.
const DATA_POLL_MS = 60 * 1000;

// useShapeData polls GET /api/data/{shape}, optionally scoped to one
// plugin instance (a card with no data_plugin_instance_id gets every
// row for the shape, same as the admin API's own default). Returns the
// last successfully fetched rows even while a later poll is failing --
// a widget should keep showing its last reading, not blank out, on a
// transient network hiccup.
export function useShapeData(shape: string, pluginInstanceID: number | null) {
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  // Every card reading a given shape re-renders on every poll otherwise
  // -- most polls return byte-identical data (a clock's rows never
  // change; weather barely does), but `setData` always handed back a
  // fresh array reference, so every consumer re-rendered anyway despite
  // nothing actually changing. On kiosk hardware (issue #31's Pi
  // performance tuning) that's real, avoidable work every 60s across
  // every card on screen. Comparing serialized content before calling
  // setState keeps the array reference (and every consumer's render)
  // stable across a no-op poll.
  const lastJSON = useRef<string>('');

  useEffect(() => {
    // A widget with no data needs at all (the clock) declares
    // dataShape: '' -- nothing to fetch, and `/api/data/` (empty shape
    // segment) isn't even a valid route.
    if (!shape) return;
    let cancelled = false;

    async function load() {
      try {
        const qs = pluginInstanceID != null ? `?plugin=${pluginInstanceID}` : '';
        const res = await fetch(`/api/data/${encodeURIComponent(shape)}${qs}`);
        if (!res.ok) return;
        const body: ShapeResponse = await res.json();
        if (cancelled) return;
        const rows = body.data ?? [];
        const json = JSON.stringify(rows);
        if (json === lastJSON.current) return;
        lastJSON.current = json;
        setData(rows);
      } catch {
        // Keep the last-known data; the display-wide reconnecting
        // overlay (driven by the layout fetch, not this one) already
        // signals connectivity loss.
      }
    }

    load();
    const interval = setInterval(load, DATA_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [shape, pluginInstanceID]);

  return data;
}

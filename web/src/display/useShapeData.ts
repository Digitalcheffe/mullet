import { useEffect, useState } from 'react';

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
        if (!cancelled) setData(body.data ?? []);
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

import { useEffect, useState } from 'react';

interface DataResponse<T> {
  data: T[];
  last_updated: string;
  source: string;
}

// Fetches rows for a data shape from the data API (see issue #14) and
// polls on the given interval.
export function usePluginData<T>(shape: string, pollMs = 30000): T[] {
  const [data, setData] = useState<T[]>([]);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await fetch(`/api/data/${shape}`);
        if (!res.ok) return;
        const body: DataResponse<T> = await res.json();
        if (!cancelled) setData(body.data);
      } catch {
        // network error; keep last known data
      }
    }

    load();
    const id = setInterval(load, pollMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [shape, pollMs]);

  return data;
}

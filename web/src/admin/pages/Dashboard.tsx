import { useEffect, useState } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './Dashboard.css';

interface DashboardStats {
  uptime_seconds: number;
  plugin_count: number;
  display_count: number;
  active_client_count: number;
}

function formatUptime(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = Math.floor(totalSeconds % 60);
  return `${h}h ${m}m ${s}s`;
}

export default function Dashboard() {
  const apiFetch = useApiFetch();
  const [stats, setStats] = useState<DashboardStats | null>(null);

  useEffect(() => {
    let cancelled = false;

    apiFetch('/api/admin/dashboard')
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error('failed to load dashboard'))))
      .then((body: DashboardStats) => {
        if (!cancelled) setStats(body);
      })
      .catch(() => {
        // leave stats null; the loading state doubles as an error state
        // for this simple overview page
      });

    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  if (!stats) {
    return <p>Loading…</p>;
  }

  return (
    <div>
      <h2>Dashboard</h2>
      <div className="stat-grid">
        <StatTile label="Uptime" value={formatUptime(stats.uptime_seconds)} />
        <StatTile label="Data Plugins" value={stats.plugin_count} />
        <StatTile label="Displays" value={stats.display_count} />
        <StatTile label="Active Clients" value={stats.active_client_count} />
      </div>
    </div>
  );
}

function StatTile({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="stat-tile">
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  );
}

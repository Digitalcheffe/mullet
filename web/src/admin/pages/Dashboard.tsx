import { useEffect, useState } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './Dashboard.css';

interface PluginStatus {
  id: number;
  plugin_id: string;
  instance_name: string;
  refresh_seconds: number;
  enabled: boolean;
  status: 'synced' | 'retrying' | 'pending' | 'disabled';
  last_error?: string;
}

interface DashboardStats {
  uptime_seconds: number;
  plugin_count: number;
  display_count: number;
  active_client_count: number;
  system_status: 'normal' | 'attention';
  plugins: PluginStatus[];
}

function formatUptime(totalSeconds: number): string {
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function formatInterval(seconds: number): string {
  if (seconds % 60 === 0) return `every ${seconds / 60} min`;
  return `every ${seconds}s`;
}

const statusLabel: Record<PluginStatus['status'], string> = {
  synced: 'Synced',
  retrying: 'Retrying',
  pending: 'Pending',
  disabled: 'Disabled',
};

export default function Dashboard() {
  const apiFetch = useApiFetch();
  const [stats, setStats] = useState<DashboardStats | null>(null);

  useEffect(() => {
    let cancelled = false;

    function load() {
      apiFetch('/api/admin/dashboard')
        .then((res) => (res.ok ? res.json() : Promise.reject(new Error('failed to load dashboard'))))
        .then((body: DashboardStats) => {
          if (!cancelled) setStats(body);
        })
        .catch(() => {});
    }

    load();
    const id = setInterval(load, 30000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [apiFetch]);

  if (!stats) {
    return <p>Loading…</p>;
  }

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <div>
          <h1>Dashboard</h1>
          <div className="dashboard-subtitle">Server overview &middot; up {formatUptime(stats.uptime_seconds)}</div>
        </div>
        <div className={`status-chip ${stats.system_status}`}>
          <span className="status-dot" />
          {stats.system_status === 'normal' ? 'All systems normal' : 'Needs attention'}
        </div>
      </div>

      <div className="stat-grid">
        <StatTile label="Uptime" value={formatUptime(stats.uptime_seconds)} />
        <StatTile label="Data Plugins" value={stats.plugin_count} suffix="active" />
        <StatTile label="Displays" value={stats.display_count} />
        <StatTile label="Active Clients" value={stats.active_client_count} suffix="online" />
      </div>

      <div className="dashboard-columns">
        <section className="dashboard-panel">
          <div className="panel-header">
            <h2>Plugin Status</h2>
          </div>
          {stats.plugins.length === 0 ? (
            <EmptyState text="No data plugins configured yet." />
          ) : (
            <div className="plugin-list">
              {stats.plugins.map((p) => (
                <div className="plugin-row" key={p.id}>
                  <div className="plugin-info">
                    <div className="plugin-name">{p.instance_name}</div>
                    <div className="plugin-detail">
                      {p.plugin_id} &middot; {formatInterval(p.refresh_seconds)}
                    </div>
                  </div>
                  <span className={`status-pill ${p.status}`}>
                    <span className="status-dot" />
                    {statusLabel[p.status]}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="dashboard-panel">
          <div className="panel-header">
            <h2>Displays</h2>
          </div>
          <EmptyState text="Displays are configured in the Designer — coming in a future update." />

          <h2 style={{ marginTop: 8 }}>Recent Activity</h2>
          <EmptyState text="Activity log coming soon." />
        </section>
      </div>
    </div>
  );
}

function StatTile({ label, value, suffix }: { label: string; value: string | number; suffix?: string }) {
  return (
    <div className="stat-tile">
      <div className="kicker">{label}</div>
      <div className="stat-value">
        {value}
        {suffix && <span className="stat-suffix">{suffix}</span>}
      </div>
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>;
}

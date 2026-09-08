import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './ServerHealthWidget.css';

// One row from GET /api/data/infrastructure -- field names match the
// shape_infrastructure columns verbatim (see internal/shapes/infrastructure.go).
export interface InfraServiceRow {
  id: string;
  plugin_instance_id: number;
  name: string;
  status: string;
  cpu_percent: number | null;
  memory_percent: number | null;
  disk_percent: number | null;
  fetched_at: string;
}

interface Config {
  showBars?: boolean;
}

const GOOD_STATUSES = new Set(['up', 'online', 'healthy', 'running']);
const BAD_STATUSES = new Set(['down', 'offline', 'error', 'stopped', 'unhealthy']);

function statusClass(status: string): string {
  const s = status.toLowerCase();
  if (GOOD_STATUSES.has(s)) return 'sh-good';
  if (BAD_STATUSES.has(s)) return 'sh-bad';
  return 'sh-warn';
}

// Resource bars use fixed load thresholds, not the status field -- a
// service can report "healthy" while still running hot enough to be
// worth flagging visually.
function barClass(percent: number): string {
  if (percent >= 90) return 'sh-bar-bad';
  if (percent >= 70) return 'sh-bar-warn';
  return 'sh-bar-good';
}

function ResourceBar({ label, percent }: { label: string; percent: number | null }) {
  if (percent == null) return null;
  return (
    <div className="sh-bar-row">
      <span className="sh-bar-label">{label}</span>
      <div className="sh-bar-track">
        <div className={`sh-bar-fill ${barClass(percent)}`} style={{ width: `${Math.min(100, Math.max(0, percent))}%` }} />
      </div>
      <span className="sh-bar-value">{Math.round(percent)}%</span>
    </div>
  );
}

function ServerHealthComponent({ data, config, size, theme }: WidgetProps<InfraServiceRow>) {
  const cfg = config as Config;
  const showBars = cfg.showBars !== false && size.h > 2;

  const style = cardStyle(theme);

  if (data.length === 0) {
    return (
      <div className="server-health-widget sh-empty" style={style}>
        No services yet
      </div>
    );
  }

  return (
    <div className="server-health-widget" style={style}>
      {data.map((svc) => (
        <div className="sh-service" key={svc.id}>
          <div className="sh-service-header">
            <span className={`sh-dot ${statusClass(svc.status)}`} />
            <span className="sh-name">{svc.name}</span>
            <span className="sh-status">{svc.status}</span>
          </div>
          {showBars && (svc.cpu_percent != null || svc.memory_percent != null || svc.disk_percent != null) && (
            <div className="sh-bars">
              <ResourceBar label="CPU" percent={svc.cpu_percent} />
              <ResourceBar label="MEM" percent={svc.memory_percent} />
              <ResourceBar label="DSK" percent={svc.disk_percent} />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

export const serverHealthPlugin: UIPlugin<InfraServiceRow> = {
  id: 'mullet-server-health',
  name: 'Server Health',
  dataShape: 'infrastructure',
  defaultSize: { w: 4, h: 5 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 10 },
  configSchema: {
    showBars: { type: 'toggle', label: 'Show resource bars', default: true },
  },
  component: ServerHealthComponent,
};

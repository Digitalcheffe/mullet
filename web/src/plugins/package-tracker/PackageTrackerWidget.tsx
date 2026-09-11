import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './PackageTrackerWidget.css';

// One row from GET /api/data/packages -- field names match the
// shape_packages columns verbatim (see internal/shapes/packages.go).
export interface PackageRow {
  id: string;
  plugin_instance_id: number;
  carrier: string;
  description: string | null;
  status: string;
  eta: string | null;
  tracking_url: string | null;
  fetched_at: string;
}

interface Config {
  showDelivered?: boolean;
}

// Package-tracking data plugins don't share a status vocabulary, so this
// only recognizes a handful of common stage keywords for the progress
// bar -- any other status still renders, just without a progress step.
const STAGES = ['ordered', 'pre_transit', 'in_transit', 'out_for_delivery', 'delivered'];

function stageIndex(status: string): number {
  const s = status.toLowerCase().replace(/\s+/g, '_');
  return STAGES.indexOf(s);
}

function isDelivered(status: string): boolean {
  return status.toLowerCase() === 'delivered';
}

// A data plugin's status string is often a machine-friendly token like
// "out_for_delivery" (matching STAGES above) -- display it as words.
function formatStatus(status: string): string {
  return status.replace(/_/g, ' ');
}

function formatETA(raw: string): string {
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function ProgressBar({ status, color }: { status: string; color: string }) {
  const idx = stageIndex(status);
  if (idx < 0) return null;
  const percent = (idx / (STAGES.length - 1)) * 100;
  return (
    <div className="pt-progress-track">
      <div className="pt-progress-fill" style={{ width: `${percent}%`, background: color }} />
    </div>
  );
}

function PackageTrackerComponent({ data, config, size, theme }: WidgetProps<PackageRow>) {
  const cfg = config as Config;
  const showDelivered = cfg.showDelivered === true;
  const compact = size.h <= 3;

  const style = cardStyle(theme);

  const visible = (showDelivered ? data : data.filter((p) => !isDelivered(p.status))).slice().sort((a, b) => {
    if (!a.eta) return 1;
    if (!b.eta) return -1;
    return a.eta.localeCompare(b.eta);
  });

  if (visible.length === 0) {
    return (
      <div className="package-tracker-widget pt-empty mullet-card" style={style}>
        No packages incoming
      </div>
    );
  }

  return (
    <div className="package-tracker-widget mullet-card" style={style}>
      {visible.map((p) => (
        <div className="pt-package" key={p.id}>
          <div className="pt-row">
            <span className="pt-carrier">{p.carrier}</span>
            <span className="pt-desc">{p.description ?? 'Package'}</span>
            {p.eta && <span className="pt-eta">{formatETA(p.eta)}</span>}
          </div>
          {!compact && (
            <>
              <ProgressBar status={p.status} color={theme.accentColor} />
              <div className="pt-status" style={{ color: theme.accentColor }}>
                {formatStatus(p.status)}
              </div>
            </>
          )}
        </div>
      ))}
    </div>
  );
}

export const packageTrackerPlugin: UIPlugin<PackageRow> = {
  id: 'mullet-package-tracker',
  name: 'Package Tracker',
  dataShape: 'packages',
  defaultSize: { w: 4, h: 5 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 10 },
  configSchema: {
    showDelivered: { type: 'toggle', label: 'Show delivered packages', default: false },
  },
  component: PackageTrackerComponent,
};

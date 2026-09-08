import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './HomeStatusWidget.css';

// One row from GET /api/data/home_devices -- field names match the
// shape_home_devices columns verbatim (see internal/shapes/home_devices.go).
export interface HomeDeviceRow {
  id: string;
  plugin_instance_id: number;
  name: string;
  area: string | null;
  device_type: string;
  state: string;
  fetched_at: string;
}

interface Config {
  showState?: boolean;
}

const DEVICE_GLYPH: Record<string, string> = {
  light: '💡',
  lock: '🔒',
  door: '🚪',
  garage: '🚗',
  sensor: '📟',
  climate: '🌡️',
};

// Tri-state read on a device's raw `state` string -- 'good'/'warn'/'bad'
// only make sense for devices with a clear "at rest" state (a locked
// door, a closed garage); anything else (sensor readings, climate
// modes, an "off" light) is 'neutral' rather than guessing at a
// judgment the framework has no basis for.
function statusClass(deviceType: string, state: string): string {
  const s = state.toLowerCase();
  switch (deviceType) {
    case 'lock':
      return s === 'locked' ? 'hs-good' : s === 'unlocked' ? 'hs-bad' : 'hs-neutral';
    case 'door':
    case 'garage':
      return s === 'closed' ? 'hs-good' : s === 'open' ? 'hs-warn' : 'hs-neutral';
    case 'light':
      return s === 'on' ? 'hs-good' : 'hs-neutral';
    default:
      return 'hs-neutral';
  }
}

function HomeStatusComponent({ data, config, size, theme }: WidgetProps<HomeDeviceRow>) {
  const cfg = config as Config;
  const showState = cfg.showState !== false;
  const compact = size.h <= 2;

  const style = cardStyle(theme);

  if (data.length === 0) {
    return (
      <div className="home-status-widget hs-empty" style={style}>
        No devices yet
      </div>
    );
  }

  const byArea = new Map<string, HomeDeviceRow[]>();
  for (const d of data) {
    const key = d.area ?? 'Other';
    if (!byArea.has(key)) byArea.set(key, []);
    byArea.get(key)!.push(d);
  }

  return (
    <div className="home-status-widget" style={style}>
      {[...byArea.entries()].map(([area, devices]) => (
        <div className="hs-group" key={area}>
          {!compact && <div className="hs-group-header">{area}</div>}
          <div className="hs-grid">
            {devices.map((d) => (
              <div className="hs-device" key={d.id}>
                <span className="hs-icon">{DEVICE_GLYPH[d.device_type] ?? '🔌'}</span>
                <span className="hs-name">{d.name}</span>
                <span className={`hs-dot ${statusClass(d.device_type, d.state)}`} />
                {showState && !compact && <span className="hs-state">{d.state}</span>}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

export const homeStatusPlugin: UIPlugin<HomeDeviceRow> = {
  id: 'mullet-home-status',
  name: 'Home Status',
  dataShape: 'home_devices',
  defaultSize: { w: 4, h: 4 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 8 },
  configSchema: {
    showState: { type: 'toggle', label: 'Show state text', default: true },
  },
  component: HomeStatusComponent,
};

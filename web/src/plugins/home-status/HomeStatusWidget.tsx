import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { DeviceIcon, MdiIcon, statusClass } from './deviceIcons';
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
  // A same-origin URL to this entity's own Home Assistant-reported
  // icon, already resolved and cached server-side (issue #175) -- null
  // if it has none, or its "mdi:xxx" name didn't resolve to a known
  // icon. Preferred over DeviceIcon's own hardcoded set when present.
  icon: string | null;
}

interface Config {
  showState?: boolean;
}

function HomeStatusComponent({ data, config, size, theme }: WidgetProps<HomeDeviceRow>) {
  const cfg = config as Config;
  const showState = cfg.showState !== false;
  const compact = size.h <= 2;

  const style = cardStyle(theme);

  if (data.length === 0) {
    return (
      <div className="home-status-widget hs-empty mullet-card" style={style}>
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
    <div className="home-status-widget mullet-card" style={style}>
      {[...byArea.entries()].map(([area, devices]) => (
        <div className="hs-group" key={area}>
          {!compact && <div className="hs-group-header">{area}</div>}
          <div className="hs-grid">
            {devices.map((d) => (
              <div className="hs-device" key={d.id}>
                <span className="hs-icon" style={{ color: theme.accentColor }}>
                  {d.icon ? (
                    <MdiIcon url={d.icon} />
                  ) : (
                    <DeviceIcon deviceType={d.device_type} width="1em" height="1em" />
                  )}
                </span>
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

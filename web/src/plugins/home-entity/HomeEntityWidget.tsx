import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { DeviceIcon, statusClass } from '../home-status/deviceIcons';
import type { HomeDeviceRow } from '../home-status/HomeStatusWidget';
import './HomeEntityWidget.css';

// The single-entity counterpart to Home Status (issue #99) -- that
// widget shows every entity the bound instance pulled in as a list;
// this one shows exactly one, picked via its own `entity` configSchema
// field, for someone who wants just the front door lock or just the
// thermostat on its own card instead of a whole list. Both read the
// same `home_devices` shape and share deviceIcons.tsx's icon/
// statusClass logic so a device reads identically in either widget.
interface Config {
  entity?: string;
  showState?: boolean;
}

function HomeEntityComponent({ data, config, theme }: WidgetProps<HomeDeviceRow>) {
  const cfg = config as Config;
  const showState = cfg.showState !== false;
  const style = cardStyle(theme);

  const device = cfg.entity ? data.find((d) => d.id === cfg.entity) : undefined;

  if (!device) {
    return (
      <div className="home-entity-widget he-empty mullet-card" style={style}>
        {cfg.entity ? 'Entity not found in the bound instance’s current data.' : 'Pick an entity in this card’s settings.'}
      </div>
    );
  }

  return (
    <div className="home-entity-widget mullet-card" style={style}>
      <span className="he-icon" style={{ color: theme.accentColor }}>
        <DeviceIcon deviceType={device.device_type} width="1em" height="1em" />
      </span>
      <span className="he-name">{device.name}</span>
      <span className="he-state-row">
        <span className={`he-dot ${statusClass(device.device_type, device.state)}`} />
        {showState && <span className="he-state">{device.state}</span>}
      </span>
    </div>
  );
}

export const homeEntityPlugin: UIPlugin<HomeDeviceRow> = {
  id: 'mullet-home-entity',
  name: 'Home Assistant Entity',
  dataShape: 'home_devices',
  defaultSize: { w: 3, h: 2 },
  minSize: { w: 2, h: 1 },
  maxSize: { w: 6, h: 4 },
  configSchema: {
    entity: {
      // dynamicField: the HA data plugin's own Discover field is named
      // "entities" (matching its SetupField key) -- this widget's own
      // config key is "entity" (singular, since it only ever holds one),
      // so the two need to be named explicitly rather than relying on
      // dynamicField defaulting to this field's own key.
      type: 'select', label: 'Entity', dynamic: true, dynamicField: 'entities',
      helpText: 'Save a Home Assistant data source on this card first, then pick which entity to show here.',
    },
    showState: { type: 'toggle', label: 'Show state text', default: true },
  },
  component: HomeEntityComponent,
};

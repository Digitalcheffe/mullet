import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { DeviceIcon, MdiIcon, statusClass } from '../home-status/deviceIcons';
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
  // A per-card custom icon (issue #174) overriding the domain-based
  // icon DeviceIcon would otherwise pick -- e.g. a photo of a specific
  // person for a presence card. Empty/unset keeps today's behavior.
  customIcon?: string;
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
        {cfg.customIcon ? (
          // A custom-uploaded icon (issue #174) stays full-color, unlike
          // MdiIcon below -- it's the admin's own image/photo, not a
          // monochrome glyph meant to be recolored to the theme.
          <img className="he-custom-icon" src={cfg.customIcon} alt="" />
        ) : device.icon ? (
          <MdiIcon url={device.icon} />
        ) : (
          <DeviceIcon deviceType={device.device_type} width="1em" height="1em" />
        )}
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
    customIcon: {
      type: 'image', label: 'Custom icon', default: '',
      helpText: 'Overrides the automatic icon for this entity. Leave unset to keep the automatic one.',
    },
  },
  component: HomeEntityComponent,
};

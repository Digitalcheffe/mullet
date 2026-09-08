import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './ExampleWidget.css';

// ExampleWidget is a reference template for writing a new UI plugin --
// see docs/plugin-development.md for the full walkthrough this backs.
// It's real, working code (compiles, type-checks) that reads the same
// `weather_current` shape the exampleplugin data plugin template
// produces, so the two can be followed end to end together.
//
// It is NOT registered in web/src/plugins/registry.ts, so it never shows
// up in the Designer's palette or renders on a real display -- copy this
// folder, rename the plugin id, and add it to registry.ts (and, if you
// want it selectable in the Designer today, DesignerPage.tsx's PALETTE)
// once your own widget is ready. That's the last step in the guide,
// deliberately not done here.

// One row from GET /api/data/weather_current -- field names match the
// shape_weather_current columns verbatim (see internal/shapes/weather.go
// and internal/db/data_api.go's generic column reader). Every UI plugin
// declares its own copy of the row shape it expects like this one does;
// there's no shared generated type between Go and TypeScript.
export interface ExampleRow {
  id: string;
  temp: number;
  condition: string;
  humidity: number | null;
  fetched_at: string;
}

// A widget's config is whatever shape its own configSchema (below)
// describes -- this one field, but a real widget can declare several.
// WidgetProps always types `config` as Record<string, unknown>, so cast
// it to your own shape like this rather than changing that generic type.
interface Config {
  unit?: string;
}

// The component itself: plain props in, JSX out, same as any other React
// component -- WidgetProps<TData> is the one contract every UI plugin's
// component must satisfy. `data` is already the typed rows GET
// /api/data/{shape} returned (empty array if none yet -- always handle
// that, a fresh instance or a slow first fetch both look like this).
function ExampleWidgetComponent({ data, config, theme }: WidgetProps<ExampleRow>) {
  const current = data[0];
  const unit = (config as Config).unit ?? '°F';
  const style = cardStyle(theme); // every widget renders its own full card chrome -- see cardStyle.ts

  if (!current) {
    return (
      <div className="example-widget ew-empty" style={style}>
        No data yet
      </div>
    );
  }

  return (
    <div className="example-widget" style={style}>
      <span className="ew-temp">
        {Math.round(current.temp)}
        {unit}
      </span>
      <span className="ew-condition" style={{ color: theme.accentColor }}>
        {current.condition}
      </span>
      {current.humidity != null && <span className="ew-humidity">💧 {current.humidity}%</span>}
    </div>
  );
}

// The manifest object a real plugin would export and add to
// web/src/plugins/registry.ts's `uiPlugins` array -- `id` must be
// unique. By convention `mullet-` prefixes only first-party plugin
// ids (see architecture.md's UI Plugins "Naming" note); leave the
// prefix off your own.
export const exampleWidgetPlugin: UIPlugin<ExampleRow> = {
  id: 'example-widget',
  name: 'Example Widget',
  dataShape: 'weather_current',
  defaultSize: { w: 4, h: 3 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 6 },
  configSchema: {
    unit: {
      type: 'select',
      label: 'Temperature unit',
      default: '°F',
      options: [
        { value: '°F', label: 'Fahrenheit' },
        { value: '°C', label: 'Celsius' },
      ],
    },
  },
  component: ExampleWidgetComponent,
};

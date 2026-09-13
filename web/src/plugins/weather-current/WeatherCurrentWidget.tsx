import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { ConditionIcon, isNightTime, type ConditionIconVariant } from '../shared/conditionIcons';
import './WeatherCurrentWidget.css';

// One row from GET /api/data/weather_current -- field names match the
// shape_weather_current columns verbatim (see internal/shapes/weather.go
// and internal/db/data_api.go's generic column scan).
export interface WeatherCurrentRow {
  id: string;
  temp: number;
  feels_like: number | null;
  condition: string;
  icon: string;
  humidity: number | null;
  high: number | null;
  low: number | null;
  sunrise: string | null;
  sunset: string | null;
  wind_speed: number | null;
  fetched_at: string;
}

type IconSize = 'small' | 'medium' | 'large';

// Base em size for the condition glyph at each setting (issue #148 --
// the default "medium" matches this widget's original fixed 1.8em, so
// existing cards look unchanged unless someone opts into small/large).
// wc-compact below still halves whichever of these is chosen, same
// ratio the old hardcoded compact rule used (1.3 / 1.8 ≈ 0.7).
const ICON_SIZE_EM: Record<IconSize, number> = { small: 1.2, medium: 1.8, large: 3.8 };

interface Config {
  unit?: string;
  showHumidity?: boolean;
  showWind?: boolean;
  iconSize?: IconSize;
  iconStyle?: ConditionIconVariant;
}

function round(n: number | null): string {
  return n == null ? '--' : String(Math.round(n));
}

function WeatherCurrentComponent({ data, config, size, theme }: WidgetProps<WeatherCurrentRow>) {
  const current = data[0];
  const cfg = config as Config;
  const unit = cfg.unit ?? '°F';
  const showHumidity = cfg.showHumidity !== false;
  const showWind = cfg.showWind !== false;
  const iconSize = cfg.iconSize ?? 'medium';
  // Grid units, not pixels -- size is the one hint a widget gets about
  // its available room (see WidgetProps). A short or narrow cell drops
  // secondary stats rather than truncating/overflowing them.
  const compact = size.h <= 2 || size.w <= 3;
  const glyphEm = ICON_SIZE_EM[iconSize] * (compact ? 0.7 : 1);

  const style = cardStyle(theme);

  if (!current) {
    return (
      <div className="weather-current-widget wc-empty mullet-card" style={style}>
        No data yet
      </div>
    );
  }

  return (
    <div className={`weather-current-widget mullet-card${compact ? ' wc-compact' : ''}`} style={style}>
      <div className="wc-main">
        <span className="wc-glyph" style={{ color: theme.accentColor, fontSize: `${glyphEm}em` }}>
          <ConditionIcon
            condition={current.condition}
            isNight={isNightTime(current.sunrise, current.sunset)}
            variant={cfg.iconStyle ?? 'outline'}
            width="1em"
            height="1em"
          />
        </span>
        <span className="wc-temp">
          {round(current.temp)}
          {unit}
        </span>
      </div>
      <div className="wc-condition" style={{ color: theme.accentColor }}>
        {current.condition}
      </div>
      {!compact && (
        <div className="wc-details">
          {(current.high != null || current.low != null) && (
            <span>
              H:{round(current.high)}
              {unit} L:{round(current.low)}
              {unit}
            </span>
          )}
          {showHumidity && current.humidity != null && <span>💧 {current.humidity}%</span>}
          {showWind && current.wind_speed != null && <span>💨 {round(current.wind_speed)}</span>}
        </div>
      )}
    </div>
  );
}

export const weatherCurrentPlugin: UIPlugin<WeatherCurrentRow> = {
  id: 'mullet-weather-current',
  name: 'Weather (Current)',
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
    showHumidity: { type: 'toggle', label: 'Show humidity', default: true },
    showWind: { type: 'toggle', label: 'Show wind', default: true },
    iconSize: {
      type: 'select',
      label: 'Icon size',
      default: 'medium',
      options: [
        { value: 'small', label: 'Small' },
        { value: 'medium', label: 'Medium' },
        { value: 'large', label: 'Large' },
      ],
    },
    iconStyle: {
      type: 'select',
      label: 'Icon style',
      default: 'outline',
      options: [
        { value: 'outline', label: 'Outline' },
        { value: 'filled', label: 'Filled / colored' },
      ],
    },
  },
  component: WeatherCurrentComponent,
};

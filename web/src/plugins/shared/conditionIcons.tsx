import type { SVGProps } from 'react';

// Real SVG icons (issue #85) replacing the old emoji map -- an emoji
// glyph renders differently per device font (Segoe UI Emoji on
// Windows, Noto Color Emoji on a Pi, Apple Color Emoji on a tablet),
// can't be recolored or crisply resized, and has no day/night variant.
// Two hand-drawn styles (issue #148): "outline" is a single-color stroke
// set matching the admin sidebar's own icon style (currentColor, so it
// takes on theme.accentColor like every other themed element); "filled"
// is a fixed, independent color palette for a more traditional weather
// app look, chosen by whoever configures the card via ConditionIconProps'
// `variant`. Both share the same viewBox/24px grid so they drop in
// interchangeably.
type IconProps = SVGProps<SVGSVGElement>;

const outlineBase = {
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

const CLOUD_PATH = 'M6.5 18.5a4 4 0 0 1-.5-7.97 5 5 0 0 1 9.6-2.35 4.5 4.5 0 0 1 1.9 8.32';

// A single teardrop, used for both outline and filled raindrops/snow
// centers -- centered on the origin, pointed end up, so callers just
// translate it into place rather than hand-coding a fresh path per drop.
function raindropPath(cx: number, cy: number, scale = 1): string {
  const r = 3.6 * scale;
  const tip = 6.4 * scale;
  return `M${cx} ${cy - tip} C${cx + r + 0.4} ${cy - r * 0.4} ${cx + r} ${cy + r} ${cx} ${cy + r} C${cx - r} ${cy + r} ${cx - r - 0.4} ${cy - r * 0.4} ${cx} ${cy - tip} Z`;
}

// ---- Outline set (enhanced -- issue #148: more rays, real teardrop
// raindrops instead of thin diagonal strokes, layered cloud puffs, a
// branched snowflake) ----

function SunIconOutline(props: IconProps) {
  const rays = Array.from({ length: 12 }, (_, i) => i * 30);
  return (
    <svg {...outlineBase} {...props}>
      <circle cx="12" cy="12" r="4.5" />
      {rays.map((angle) => {
        const long = angle % 60 === 0;
        const inner = long ? 6.3 : 6.8;
        const outer = long ? 9.6 : 8.7;
        return <line key={angle} x1="12" y1={12 - inner} x2="12" y2={12 - outer} transform={`rotate(${angle} 12 12)`} />;
      })}
    </svg>
  );
}

function MoonIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a6.8 6.8 0 0 0 10.5 10.5z" />
      <circle cx="9" cy="8.5" r="0.6" fill="currentColor" stroke="none" />
      <circle cx="12.5" cy="12.5" r="0.4" fill="currentColor" stroke="none" />
    </svg>
  );
}

function CloudIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d="M4.5 14.2a3 3 0 0 1 .4-6 4 4 0 0 1 7.3-2.1" opacity="0.55" />
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

function CloudSunIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <circle cx="16.5" cy="7.5" r="3" />
      <path d="M16.5 2.5v1.4M21.5 7.5h-1.4M20 4l-1 1M20 11l-1-1" />
      <path d="M4.5 14.2a3 3 0 0 1 .4-6 4 4 0 0 1 7.3-2.1" opacity="0.55" />
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

function CloudMoonIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d="M20.5 5.3a4.3 4.3 0 1 1-5.3 5.3 3.4 3.4 0 0 0 5.3-5.3z" />
      <path d="M4.5 14.2a3 3 0 0 1 .4-6 4 4 0 0 1 7.3-2.1" opacity="0.55" />
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

function DrizzleIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d={CLOUD_PATH} />
      <path d={raindropPath(9, 20.5, 0.65)} fill="currentColor" stroke="none" />
      <path d={raindropPath(13, 20.5, 0.65)} fill="currentColor" stroke="none" />
    </svg>
  );
}

function RainIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d={CLOUD_PATH} />
      <path d={raindropPath(7.5, 20.5, 0.75)} fill="currentColor" stroke="none" />
      <path d={raindropPath(12, 21.3, 0.75)} fill="currentColor" stroke="none" />
      <path d={raindropPath(16.5, 20.5, 0.75)} fill="currentColor" stroke="none" />
    </svg>
  );
}

// One 6-armed snowflake, each arm carrying a small fork -- shared by the
// outline (currentColor stroke) and filled (fixed stroke color) sets.
function snowflakeArms(cx: number, cy: number, r: number) {
  return Array.from({ length: 3 }, (_, i) => i * 60).map((angle) => (
    <g key={angle} transform={`rotate(${angle} ${cx} ${cy})`}>
      <line x1={cx} y1={cy - r} x2={cx} y2={cy + r} />
      <line x1={cx} y1={cy - r * 0.55} x2={cx - r * 0.3} y2={cy - r * 0.8} />
      <line x1={cx} y1={cy - r * 0.55} x2={cx + r * 0.3} y2={cy - r * 0.8} />
      <line x1={cx} y1={cy + r * 0.55} x2={cx - r * 0.3} y2={cy + r * 0.8} />
      <line x1={cx} y1={cy + r * 0.55} x2={cx + r * 0.3} y2={cy + r * 0.8} />
    </g>
  ));
}

function SnowIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d={CLOUD_PATH} />
      <g strokeWidth="1.2">
        {snowflakeArms(8, 20.5, 2.6)}
        {snowflakeArms(15, 20.5, 2.6)}
      </g>
    </svg>
  );
}

function ThunderstormIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d={CLOUD_PATH} />
      <path d="M12.5 15.5 9.8 20h3l-1.6 3.5 4.5-5.3h-2.7z" fill="currentColor" strokeLinejoin="round" />
    </svg>
  );
}

function FogIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d="M4 9.5h9" opacity="0.9" />
      <path d="M15.5 9.5h4.5" opacity="0.5" />
      <path d="M6.5 13.5h13" opacity="0.7" />
      <path d="M4 17.5h6" opacity="0.5" />
      <path d="M12 17.5h8" opacity="0.9" />
      <path d="M8 21h9" opacity="0.6" />
    </svg>
  );
}

function ThermometerIconOutline(props: IconProps) {
  return (
    <svg {...outlineBase} {...props}>
      <path d="M12 14.8V5a2 2 0 1 0-4 0v9.8a4 4 0 1 0 4 0z" />
      <path d="M10 6.5v7.3" />
    </svg>
  );
}

// ---- Filled set (issue #148) -- a fixed, independent color palette
// rather than currentColor, for a more traditional colored weather-app
// look. Each icon keeps a thin stroke in a darker shade of its own fill
// for definition against light and dark backgrounds alike, rather than
// relying on the surrounding theme. ----

function SunIconFilled(props: IconProps) {
  const rays = [0, 45, 90, 135, 180, 225, 270, 315];
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      {rays.map((angle) => (
        <line key={angle} x1="12" y1="2.6" x2="12" y2="5.6" stroke="#F5A623" strokeWidth="2" strokeLinecap="round" transform={`rotate(${angle} 12 12)`} />
      ))}
      <circle cx="12" cy="12" r="5.2" fill="#FFC542" stroke="#F5A623" strokeWidth="1" />
    </svg>
  );
}

function MoonIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a6.8 6.8 0 0 0 10.5 10.5z" fill="#CBD5E1" stroke="#94A3B8" strokeWidth="1" />
      <circle cx="9" cy="8.5" r="0.7" fill="#94A3B8" />
      <circle cx="12.7" cy="12.7" r="0.5" fill="#94A3B8" />
    </svg>
  );
}

function CloudIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d={`${CLOUD_PATH}H6.5z`} fill="#B0BEC5" stroke="#90A4AE" strokeWidth="1" />
    </svg>
  );
}

function CloudSunIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <g opacity="0.95">
        <line x1="16.5" y1="1.6" x2="16.5" y2="3.4" stroke="#F5A623" strokeWidth="1.6" strokeLinecap="round" />
        <line x1="22.4" y1="7.5" x2="20.6" y2="7.5" stroke="#F5A623" strokeWidth="1.6" strokeLinecap="round" />
        <line x1="20.5" y1="3.5" x2="19.3" y2="4.7" stroke="#F5A623" strokeWidth="1.6" strokeLinecap="round" />
        <circle cx="16.5" cy="7.5" r="3.2" fill="#FFC542" stroke="#F5A623" strokeWidth="1" />
      </g>
      <path d={`${CLOUD_PATH}H6.5z`} fill="#B0BEC5" stroke="#90A4AE" strokeWidth="1" />
    </svg>
  );
}

function CloudMoonIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d="M20.5 5.3a4.3 4.3 0 1 1-5.3 5.3 3.4 3.4 0 0 0 5.3-5.3z" fill="#CBD5E1" stroke="#94A3B8" strokeWidth="1" />
      <path d={`${CLOUD_PATH}H6.5z`} fill="#B0BEC5" stroke="#90A4AE" strokeWidth="1" />
    </svg>
  );
}

function DrizzleIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d={CLOUD_PATH} fill="#90A4AE" stroke="#78909C" strokeWidth="1" />
      <path d={raindropPath(9, 20.5, 0.65)} fill="#4FC3F7" />
      <path d={raindropPath(13, 20.5, 0.65)} fill="#4FC3F7" />
    </svg>
  );
}

function RainIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d={CLOUD_PATH} fill="#78909C" stroke="#607D8B" strokeWidth="1" />
      <path d={raindropPath(7.5, 20.5, 0.75)} fill="#29B6F6" />
      <path d={raindropPath(12, 21.3, 0.75)} fill="#29B6F6" />
      <path d={raindropPath(16.5, 20.5, 0.75)} fill="#29B6F6" />
    </svg>
  );
}

function SnowIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="#90A4AE" strokeWidth="1.2" {...props}>
      <path d={CLOUD_PATH} fill="#90A4AE" stroke="#78909C" strokeWidth="1" />
      <g stroke="#E1F5FE">
        {snowflakeArms(8, 20.5, 2.6)}
        {snowflakeArms(15, 20.5, 2.6)}
      </g>
    </svg>
  );
}

function ThunderstormIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d={CLOUD_PATH} fill="#607D8B" stroke="#546E7A" strokeWidth="1" />
      <path d="M12.5 15.5 9.8 20h3l-1.6 3.5 4.5-5.3h-2.7z" fill="#FFD600" stroke="#F5A623" strokeWidth="0.6" strokeLinejoin="round" />
    </svg>
  );
}

function FogIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d="M4 9.5h9" stroke="#B0BEC5" strokeWidth="1.6" strokeLinecap="round" opacity="0.9" />
      <path d="M15.5 9.5h4.5" stroke="#B0BEC5" strokeWidth="1.6" strokeLinecap="round" opacity="0.5" />
      <path d="M6.5 13.5h13" stroke="#90A4AE" strokeWidth="1.6" strokeLinecap="round" opacity="0.8" />
      <path d="M4 17.5h6" stroke="#B0BEC5" strokeWidth="1.6" strokeLinecap="round" opacity="0.5" />
      <path d="M12 17.5h8" stroke="#90A4AE" strokeWidth="1.6" strokeLinecap="round" opacity="0.9" />
      <path d="M8 21h9" stroke="#B0BEC5" strokeWidth="1.6" strokeLinecap="round" opacity="0.6" />
    </svg>
  );
}

function ThermometerIconFilled(props: IconProps) {
  return (
    <svg {...outlineBase} stroke="none" {...props}>
      <path d="M12 14.8V5a2 2 0 1 0-4 0v9.8a4 4 0 1 0 4 0z" fill="#ECEFF1" stroke="#90A4AE" strokeWidth="1" />
      <circle cx="10" cy="17" r="2.3" fill="#EF5350" />
      <path d="M10 6.5v9" stroke="#EF5350" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

// isNightTime decides which of a day/night icon pair to show, from a
// row's own sunrise/sunset ("HH:MM", see internal/shapes/weather.go).
// Compares against local wall-clock time as a plain "HH:MM" string,
// which sorts correctly within a single day the same way the values
// themselves do. Falls back to day icons (false) when either is
// missing -- most weather plugins populate both, but nothing should
// crash or show a wrong-looking icon over a field that's simply absent.
export function isNightTime(sunrise: string | null | undefined, sunset: string | null | undefined): boolean {
  if (!sunrise || !sunset) return false;
  const now = new Date();
  const hhmm = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`;
  return hhmm < sunrise || hhmm >= sunset;
}

export type ConditionIconVariant = 'outline' | 'filled';

export interface ConditionIconProps extends IconProps {
  condition: string | undefined | null;
  // Omit when a condition has no meaningful time-of-day (a forecast row
  // covers a whole day, not one moment) -- always renders the day icon.
  isNight?: boolean;
  // 'outline' (default) takes on theme.accentColor via currentColor,
  // matching every other themed element; 'filled' uses its own fixed
  // color palette instead (issue #148).
  variant?: ConditionIconVariant;
}

// The direct replacement for the old conditionGlyph(condition): string
// -- a component instead of a string, since there's no other way to
// hand back "this SVG, tinted whatever color the caller wants."
export function ConditionIcon({ condition, isNight = false, variant = 'outline', ...props }: ConditionIconProps) {
  const filled = variant === 'filled';
  switch ((condition ?? '').toLowerCase()) {
    case 'clear':
      if (isNight) return filled ? <MoonIconFilled {...props} /> : <MoonIconOutline {...props} />;
      return filled ? <SunIconFilled {...props} /> : <SunIconOutline {...props} />;
    case 'clouds':
      if (isNight) return filled ? <CloudMoonIconFilled {...props} /> : <CloudMoonIconOutline {...props} />;
      return filled ? <CloudSunIconFilled {...props} /> : <CloudSunIconOutline {...props} />;
    case 'drizzle':
      return filled ? <DrizzleIconFilled {...props} /> : <DrizzleIconOutline {...props} />;
    case 'rain':
      return filled ? <RainIconFilled {...props} /> : <RainIconOutline {...props} />;
    case 'snow':
      return filled ? <SnowIconFilled {...props} /> : <SnowIconOutline {...props} />;
    case 'thunderstorm':
      return filled ? <ThunderstormIconFilled {...props} /> : <ThunderstormIconOutline {...props} />;
    case 'fog':
    case 'mist':
    case 'haze':
    case 'smoke':
      return filled ? <FogIconFilled {...props} /> : <FogIconOutline {...props} />;
    default:
      return filled ? <ThermometerIconFilled {...props} /> : <ThermometerIconOutline {...props} />;
  }
}

// CloudIcon isn't reachable through ConditionIcon (the plugins never
// emit a plain "overcast" category distinct from "clouds"), but stays
// exported for a future condition category or another widget that
// wants a plain cloud without the sun/moon.
export { CloudIconOutline as CloudIcon, CloudIconFilled };

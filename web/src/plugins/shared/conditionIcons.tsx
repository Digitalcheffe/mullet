import type { SVGProps } from 'react';

// Real SVG icons (issue #85) replacing the old emoji map -- an emoji
// glyph renders differently per device font (Segoe UI Emoji on
// Windows, Noto Color Emoji on a Pi, Apple Color Emoji on a tablet),
// can't be recolored or crisply resized, and has no day/night variant.
// Hand-drawn stroke icons instead, matching the admin sidebar's own
// icon style (web/src/admin/layout/icons.tsx) -- `stroke="currentColor"`
// so an icon takes on whatever color its caller sets (theme.accentColor
// throughout this app's widgets), and `width`/`height` are left for the
// caller to set in `em` so an icon scales with its surrounding
// font-size exactly like the emoji it replaces did.
type IconProps = SVGProps<SVGSVGElement>;

const base = {
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

function SunIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="12" r="4.5" />
      <path d="M12 2.5v3M12 18.5v3M21.5 12h-3M5.5 12h-3M18.4 5.6l-2.1 2.1M7.7 16.3l-2.1 2.1M18.4 18.4l-2.1-2.1M7.7 7.7 5.6 5.6" />
    </svg>
  );
}

function MoonIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a6.8 6.8 0 0 0 10.5 10.5z" />
    </svg>
  );
}

// The base puff shared by every cloud-bearing icon below -- kept as
// plain path data (not a reusable sub-component) since SVG has no
// straightforward way to share a <path> across siblings without also
// pulling in <defs>/<use>, which is more machinery than four inlined
// copies of one path justify here.
const CLOUD_PATH = 'M6.5 18.5a4 4 0 0 1-.5-7.97 5 5 0 0 1 9.6-2.35 4.5 4.5 0 0 1 1.9 8.32';

function CloudIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

// "Clouds" is the only cloud-ish category the two weather data plugins
// normalize to (see conditionGlyph's old doc comment) -- treated as
// "partly cloudy" for icon purposes, so it gets the sun/moon-peeking
// treatment issue #85 asks for rather than a flat overcast icon.
function CloudSunIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="16.5" cy="7.5" r="3" />
      <path d="M16.5 2.5v1.4M21.5 7.5h-1.4M20 4l-1 1M20 11l-1-1" />
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

function CloudMoonIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M20.5 5.3a4.3 4.3 0 1 1-5.3 5.3 3.4 3.4 0 0 0 5.3-5.3z" />
      <path d={`${CLOUD_PATH}H6.5z`} />
    </svg>
  );
}

function DrizzleIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d={CLOUD_PATH} />
      <path d="M9 19.5l-1 2M13 19.5l-1 2" />
    </svg>
  );
}

function RainIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d={CLOUD_PATH} />
      <path d="M8 19l-1.3 2.6M12 19l-1.3 2.6M16 19l-1.3 2.6" />
    </svg>
  );
}

function SnowIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d={CLOUD_PATH} />
      <path d="M8 19.3v3M6.6 20.1l2.8 1.4M9.4 20.1l-2.8 1.4" />
      <path d="M15 19.3v3M13.6 20.1l2.8 1.4M16.4 20.1l-2.8 1.4" />
    </svg>
  );
}

function ThunderstormIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d={CLOUD_PATH} />
      <path d="M12.5 15.5 9.8 20h3l-1.6 3.5 4.5-5.3h-2.7z" strokeLinejoin="round" />
    </svg>
  );
}

function FogIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M4 10.5h11M17.5 10.5h2.5M6.5 14.5h13M4 18.5h11M17.5 18.5h2.5" />
    </svg>
  );
}

function ThermometerIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M12 14.8V5a2 2 0 1 0-4 0v9.8a4 4 0 1 0 4 0z" />
      <path d="M10 6.5v7.3" />
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

export interface ConditionIconProps extends IconProps {
  condition: string | undefined | null;
  // Omit when a condition has no meaningful time-of-day (a forecast row
  // covers a whole day, not one moment) -- always renders the day icon.
  isNight?: boolean;
}

// The direct replacement for the old conditionGlyph(condition): string
// -- a component instead of a string, since there's no other way to
// hand back "this SVG, tinted whatever color the caller wants."
export function ConditionIcon({ condition, isNight = false, ...props }: ConditionIconProps) {
  switch ((condition ?? '').toLowerCase()) {
    case 'clear':
      return isNight ? <MoonIcon {...props} /> : <SunIcon {...props} />;
    case 'clouds':
      return isNight ? <CloudMoonIcon {...props} /> : <CloudSunIcon {...props} />;
    case 'drizzle':
      return <DrizzleIcon {...props} />;
    case 'rain':
      return <RainIcon {...props} />;
    case 'snow':
      return <SnowIcon {...props} />;
    case 'thunderstorm':
      return <ThunderstormIcon {...props} />;
    case 'fog':
    case 'mist':
    case 'haze':
    case 'smoke':
      return <FogIcon {...props} />;
    default:
      return <ThermometerIcon {...props} />;
  }
}

// CloudIcon isn't reachable through ConditionIcon (the plugins never
// emit a plain "overcast" category distinct from "clouds"), but stays
// exported for a future condition category or another widget that
// wants a plain cloud without the sun/moon.
export { CloudIcon };

// Minimal inline nav icons matching the admin-ui-dashboard.html mockup's
// hand-drawn SVG style. Kept as plain components rather than pulling in
// an icon library for a handful of glyphs.
import type { SVGProps } from 'react';

type IconProps = SVGProps<SVGSVGElement>;

const base = {
  width: 18,
  height: 18,
  viewBox: '0 0 20 20',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.6,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

export function DashboardIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="3" y="3" width="6" height="6" rx="1" />
      <rect x="11" y="3" width="6" height="6" rx="1" />
      <rect x="3" y="11" width="6" height="6" rx="1" />
      <rect x="11" y="11" width="6" height="6" rx="1" />
    </svg>
  );
}

export function PluginsIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M8 3v4M12 3v4M6 7h8v3a4 4 0 0 1-4 4 4 4 0 0 1-4-4V7z" />
      <path d="M10 14v3" />
    </svg>
  );
}

export function DisplaysIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="3" y="4" width="14" height="12" rx="2" />
      <line x1="3" y1="8" x2="17" y2="8" />
    </svg>
  );
}

export function ThemesIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="10" cy="10" r="7" />
      <circle cx="7.3" cy="8" r="1" />
      <circle cx="10" cy="6" r="1" />
      <circle cx="12.7" cy="8" r="1" />
    </svg>
  );
}

export function ClientsIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="7" cy="7" r="3" />
      <path d="M2.3 17c0-3 2.1-5 4.7-5s4.7 2 4.7 5" />
      <circle cx="15" cy="8.3" r="2.3" />
      <path d="M13 17c.3-2 1.5-3.4 3-3.6" />
    </svg>
  );
}

export function SettingsIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="10" cy="10" r="2.6" />
      <path d="M10 3v2M10 15v2M17 10h-2M5 10H3M14.6 5.4l-1.3 1.3M6.7 13.3l-1.3 1.3M14.6 14.6l-1.3-1.3M6.7 6.7L5.4 5.4" />
    </svg>
  );
}

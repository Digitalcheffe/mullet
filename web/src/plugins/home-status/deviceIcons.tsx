import type { ComponentType, SVGProps } from 'react';

// Real SVG icons (issue #85) replacing HomeStatusWidget's old emoji map
// -- see conditionIcons.tsx's own doc comment for why. Same hand-drawn
// stroke style and the same `stroke="currentColor"` + caller-sized
// `em` convention.
type IconProps = SVGProps<SVGSVGElement>;

const base = {
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

function LightIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M9 21h6M10 18.5h4" />
      <path d="M12 3a6 6 0 0 0-3.5 10.9c.6.45 1 1.15 1.1 1.9l.1.7h4.6l.1-.7c.1-.75.5-1.45 1.1-1.9A6 6 0 0 0 12 3z" />
    </svg>
  );
}

function LockIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="5" y="11" width="14" height="10" rx="2" />
      <path d="M8 11V7.5a4 4 0 0 1 8 0V11" />
      <circle cx="12" cy="16" r="1.3" />
    </svg>
  );
}

function DoorIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <rect x="6" y="2.5" width="12" height="19" rx="1" />
      <circle cx="14.5" cy="12" r="0.9" fill="currentColor" stroke="none" />
    </svg>
  );
}

function GarageIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M3 21V10.5L12 4l9 6.5V21" />
      <path d="M3 21h18M6 21v-8h12v8" />
      <path d="M6 15h12M6 17.7h12" />
    </svg>
  );
}

function SensorIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="12" r="2" />
      <path d="M8.5 8.5a5 5 0 0 0 0 7M15.5 8.5a5 5 0 0 1 0 7M5.3 5.3a9.5 9.5 0 0 0 0 13.4M18.7 5.3a9.5 9.5 0 0 1 0 13.4" />
    </svg>
  );
}

function ClimateIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M12 14.8V5a2 2 0 1 0-4 0v9.8a4 4 0 1 0 4 0z" />
      <path d="M10 6.5v7.3" />
    </svg>
  );
}

function PlugIcon(props: IconProps) {
  return (
    <svg {...base} {...props}>
      <path d="M9 2.5v5M15 2.5v5" />
      <path d="M6.5 7.5h11v4a5.5 5.5 0 0 1-11 0v-4z" />
      <path d="M12 17v4.5" />
    </svg>
  );
}

const DEVICE_ICONS: Record<string, ComponentType<IconProps>> = {
  light: LightIcon,
  lock: LockIcon,
  door: DoorIcon,
  garage: GarageIcon,
  sensor: SensorIcon,
  climate: ClimateIcon,
};

export function DeviceIcon({ deviceType, ...props }: { deviceType: string } & IconProps) {
  const Icon = DEVICE_ICONS[deviceType] ?? PlugIcon;
  return <Icon {...props} />;
}

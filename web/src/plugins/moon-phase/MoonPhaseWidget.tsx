import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './MoonPhaseWidget.css';

const SYNODIC_MONTH_DAYS = 29.530588853;
// A known new moon instant, used as the epoch for the age calculation
// below -- any real new moon works as the reference point since the
// synodic month is constant.
const REFERENCE_NEW_MOON = Date.UTC(2000, 0, 6, 18, 14);

const PHASE_NAMES = [
  'New Moon',
  'Waxing Crescent',
  'First Quarter',
  'Waxing Gibbous',
  'Full Moon',
  'Waning Gibbous',
  'Last Quarter',
  'Waning Crescent',
];

// Fraction of the way through the current synodic month, in [0, 1) --
// 0 and just under 1 are both "new moon", 0.5 is full moon. Pure
// calculation from the date, no external API or data plugin needed
// (issue #103).
function phaseFraction(date: Date): number {
  const ageDays = (date.getTime() - REFERENCE_NEW_MOON) / 86_400_000;
  const cycles = ageDays / SYNODIC_MONTH_DAYS;
  return cycles - Math.floor(cycles);
}

function phaseName(p: number): string {
  return PHASE_NAMES[Math.round(p * 8) % 8];
}

// A simple two-arc "lune" construction: an outer semicircle on the lit
// side, closed by an inner arc whose horizontal radius shrinks toward 0
// at half-lit (a flat diameter) and grows back toward the full radius
// at new/full -- bulging toward the same side as the outer arc for a
// crescent (less than half lit) or the opposite side for a gibbous
// (more than half lit). This traces a crescent, a half-disc, or a
// gibbous continuously as illumination varies, without needing 8
// separately hand-drawn icon shapes.
function moonIconPath(p: number): string {
  const r = 10;
  const cx = 12;
  const cy = 12;
  const angle = p * 2 * Math.PI;
  const illumination = (1 - Math.cos(angle)) / 2;
  const waxing = p < 0.5;
  const outerSweep = waxing ? 1 : 0;
  const gibbous = illumination > 0.5;
  // The inner arc runs bottom-to-top, the reverse of the outer arc's
  // top-to-bottom -- reversing an arc's endpoints flips which side of
  // the chord a given sweep-flag traces, so matching the outer arc's
  // side (crescent) needs the *opposite* flag, and bulging the
  // opposite side (gibbous) needs the *same* flag.
  const innerSweep = gibbous ? outerSweep : outerSweep === 1 ? 0 : 1;
  const rx = r * Math.abs(1 - 2 * illumination);
  return `M ${cx} ${cy - r} A ${r} ${r} 0 0 ${outerSweep} ${cx} ${cy + r} A ${rx} ${r} 0 0 ${innerSweep} ${cx} ${cy - r} Z`;
}

function MoonPhaseComponent({ theme }: WidgetProps<unknown>) {
  const p = phaseFraction(new Date());
  const style = cardStyle(theme);

  return (
    <div className="moon-phase-widget mullet-card" style={style}>
      <svg className="moon-phase-icon" viewBox="0 0 24 24" role="img" aria-label={phaseName(p)}>
        <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="1.5" opacity="0.4" />
        <path d={moonIconPath(p)} fill="currentColor" style={{ color: theme.accentColor }} />
      </svg>
      <div className="moon-phase-name">{phaseName(p)}</div>
    </div>
  );
}

export const moonPhasePlugin: UIPlugin<unknown> = {
  id: 'mullet-moon-phase',
  name: 'Moon Phase',
  dataShape: '',
  defaultSize: { w: 2, h: 2 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 4, h: 4 },
  component: MoonPhaseComponent,
};

import { ColorField } from './ThemeTokenFields';
import './GradientStopEditor.css';

interface Stop {
  color: string;
  position: number;
}

// Splits on commas that aren't nested inside parentheses -- a plain
// split(',') would break a stop like "rgba(255, 255, 255, 0.06) 50%"
// apart at its own internal commas.
function splitTopLevel(value: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let current = '';
  for (const ch of value) {
    if (ch === '(') depth++;
    if (ch === ')') depth--;
    if (ch === ',' && depth === 0) {
      parts.push(current.trim());
      current = '';
    } else {
      current += ch;
    }
  }
  if (current.trim()) parts.push(current.trim());
  return parts;
}

// Parses a `linear-gradient(<angle>deg, <color> <pos>%, ...)` string
// into an angle plus its stops. Returns null for anything else (a plain
// solid color, a malformed string, an unsupported gradient shape) so the
// caller can fall back to a sane default instead of rendering garbage.
function parseGradient(value: string): { angle: number; stops: Stop[] } | null {
  const match = value.match(/^linear-gradient\(\s*([\d.]+)deg\s*,\s*(.+)\)\s*$/);
  if (!match) return null;
  const angle = Number(match[1]);
  const stops: Stop[] = [];
  for (const part of splitTopLevel(match[2])) {
    const stopMatch = part.match(/^(.+?)\s+([\d.]+)%$/);
    if (!stopMatch) return null;
    stops.push({ color: stopMatch[1].trim(), position: Number(stopMatch[2]) });
  }
  if (stops.length < 2) return null;
  return { angle, stops };
}

function composeGradient(angle: number, stops: Stop[]): string {
  // CSS requires stops in non-decreasing position order -- a browser
  // clamps anything out of order up to match, which would silently
  // misplace a stop rather than error. Stops are kept in whatever order
  // the admin added them for the editable rows below (resorting rows out
  // from under someone mid-edit would be its own kind of confusing), so
  // this sorts only the composed output string, not the `stops` array
  // itself.
  const sorted = [...stops].sort((a, b) => a.position - b.position);
  return `linear-gradient(${angle}deg, ${sorted.map((s) => `${s.color} ${s.position}%`).join(', ')})`;
}

const FALLBACK: { angle: number; stops: Stop[] } = {
  angle: 160,
  stops: [
    { color: '#0b0f14', position: 0 },
    { color: '#1b2740', position: 100 },
  ],
};

export default function GradientStopEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const parsed = parseGradient(value) ?? FALLBACK;
  const { angle, stops } = parsed;

  function update(nextAngle: number, nextStops: Stop[]) {
    onChange(composeGradient(nextAngle, nextStops));
  }

  function updateStop(index: number, patch: Partial<Stop>) {
    const next = stops.map((s, i) => (i === index ? { ...s, ...patch } : s));
    update(angle, next);
  }

  function addStop() {
    // Inserted at the midpoint of the widest gap between existing stops,
    // rather than always at the end -- adding a stop after the last one
    // (already at 100%) would otherwise collapse two stops on top of
    // each other.
    let gapStart = 0;
    let gapEnd = 100;
    let widest = -1;
    const sorted = [...stops].sort((a, b) => a.position - b.position);
    for (let i = 0; i < sorted.length - 1; i++) {
      const gap = sorted[i + 1].position - sorted[i].position;
      if (gap > widest) {
        widest = gap;
        gapStart = sorted[i].position;
        gapEnd = sorted[i + 1].position;
      }
    }
    const position = Math.round((gapStart + gapEnd) / 2);
    update(angle, [...stops, { color: '#ffffff', position }]);
  }

  function removeStop(index: number) {
    if (stops.length <= 2) return;
    update(
      angle,
      stops.filter((_, i) => i !== index),
    );
  }

  return (
    <div className="gradient-editor">
      <div className="gradient-preview" style={{ background: composeGradient(angle, stops) }} />

      <label className="field gradient-angle-field">
        <span className="kicker">Angle</span>
        <div className="gradient-angle-row">
          <input
            type="range"
            min={0}
            max={360}
            value={angle}
            onChange={(e) => update(Number(e.target.value), stops)}
          />
          <span className="gradient-angle-value">{angle}°</span>
        </div>
      </label>

      <div className="gradient-stops">
        {stops.map((stop, i) => (
          <div className="gradient-stop-row" key={i}>
            <ColorField value={stop.color} onChange={(v) => updateStop(i, { color: v })} />
            <input
              type="number"
              className="gradient-stop-position"
              min={0}
              max={100}
              value={stop.position}
              onChange={(e) => updateStop(i, { position: Number(e.target.value) })}
              aria-label={`Stop ${i + 1} position`}
            />
            <span className="gradient-stop-percent">%</span>
            <button
              type="button"
              className="gradient-stop-remove"
              onClick={() => removeStop(i)}
              disabled={stops.length <= 2}
              aria-label={`Remove stop ${i + 1}`}
              title={stops.length <= 2 ? 'A gradient needs at least two stops' : 'Remove this stop'}
            >
              ×
            </button>
          </div>
        ))}
      </div>

      <button type="button" className="btn-secondary gradient-add-stop" onClick={addStop}>
        + Add stop
      </button>
    </div>
  );
}

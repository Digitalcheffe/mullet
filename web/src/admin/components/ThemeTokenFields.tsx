import { useEffect, useRef, useState, type CSSProperties, type PointerEvent } from 'react';
import type { ThemeTokens } from '../../shared/themes/tokens';
import { FONT_OPTIONS } from '../../shared/themes/fontOptions';
import GradientStopEditor from './GradientStopEditor';
import './ThemeTokenFields.css';

type ThemeTokenKey = keyof ThemeTokens;

const ALL_FIELDS: ThemeTokenKey[] = [
  'background',
  'cardBackground',
  'cardBorder',
  'cardStyle',
  'textColor',
  'accentColor',
  'successColor',
  'warningColor',
  'errorColor',
  'infoColor',
  'fontFamily',
  'headingFontFamily',
  'fontSize',
  'fontSizeSmall',
  'fontSizeLarge',
  'borderRadius',
  'opacity',
  'blur',
];

interface ThemeTokenFieldsProps {
  values: Partial<ThemeTokens>;
  onChange: (values: Partial<ThemeTokens>) => void;
  // Restricts which tokens render -- used for the card-level override
  // editor, which is meant to stay narrow (background opacity, accent
  // color) rather than let one card override a display's whole look.
  // Omit for the full set, used by the Theme editor itself.
  fields?: ThemeTokenKey[];
  // Uploads a file (issue #58) and resolves to the URL it's now
  // reachable at, for the "Image" background type's "Upload image"
  // button -- omitted entirely (no button rendered) by the Designer's
  // narrow card-override editor, which never shows the background
  // field in the first place; only the full Theme editor passes this.
  onUploadImage?: (file: File) => Promise<string>;
}

export default function ThemeTokenFields({ values, onChange, fields = ALL_FIELDS, onUploadImage }: ThemeTokenFieldsProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  function set<K extends ThemeTokenKey>(key: K, value: ThemeTokens[K]) {
    onChange({ ...values, [key]: value });
  }

  const show = (key: ThemeTokenKey) => fields.includes(key);

  async function handleFileSelected(file: File) {
    if (!onUploadImage) return;
    setUploadError(null);
    setUploading(true);
    try {
      const url = await onUploadImage(file);
      set('background', { type: 'image', value: url });
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  }

  return (
    <div className="theme-token-fields">
      {show('background') && (
        <label className="field">
          <span className="kicker">Background</span>
          <div className="background-field-row">
            <select
              value={values.background?.type ?? 'solid'}
              onChange={(e) => {
                const type = e.target.value as ThemeTokens['background']['type'];
                const current = values.background?.value ?? '';
                // Switching into "Gradient" from a plain solid color (or
                // an empty value) needs a real gradient string to hand
                // the stop editor below -- otherwise it'd have nothing
                // parseable to show. Reuses the existing color as the
                // gradient's first stop rather than discarding it.
                const value =
                  type === 'gradient' && !/^linear-gradient\(/.test(current)
                    ? `linear-gradient(160deg, ${current || '#0b0f14'} 0%, #1b2740 100%)`
                    : current;
                set('background', { type, value });
              }}
            >
              <option value="solid">Solid</option>
              <option value="gradient">Gradient</option>
              <option value="image">Image URL</option>
            </select>
            {values.background?.type !== 'gradient' && (
              <input
                value={values.background?.value ?? ''}
                onChange={(e) => set('background', { type: values.background?.type ?? 'solid', value: e.target.value })}
                placeholder={values.background?.type === 'image' ? 'https://... or upload one' : '#0b0f14'}
              />
            )}
            {values.background?.type === 'image' && onUploadImage && (
              <>
                <button type="button" className="btn-secondary" onClick={() => fileInputRef.current?.click()} disabled={uploading}>
                  {uploading ? 'Uploading…' : 'Upload image'}
                </button>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  hidden
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    e.target.value = ''; // allow re-selecting the same file later
                    if (file) handleFileSelected(file);
                  }}
                />
              </>
            )}
          </div>
          {values.background?.type === 'gradient' && (
            <GradientStopEditor
              value={values.background.value}
              onChange={(v) => set('background', { type: 'gradient', value: v })}
            />
          )}
          {uploadError && (
            <span className="field-help" style={{ color: 'var(--error)' }}>
              {uploadError}
            </span>
          )}
        </label>
      )}

      {show('cardBackground') && (
        <ColorField label="Card background" value={values.cardBackground ?? ''} onChange={(v) => set('cardBackground', v)} />
      )}
      {show('cardBorder') && <ColorField label="Card border" value={values.cardBorder ?? ''} onChange={(v) => set('cardBorder', v)} />}
      {show('cardStyle') && (
        <label className="field">
          <span className="kicker">Card style</span>
          <select value={values.cardStyle ?? 'glass'} onChange={(e) => set('cardStyle', e.target.value as ThemeTokens['cardStyle'])}>
            <option value="glass">Glass (translucent, blurred)</option>
            <option value="solid">Solid (flat info block)</option>
          </select>
        </label>
      )}
      {show('textColor') && <ColorField label="Text color" value={values.textColor ?? ''} onChange={(v) => set('textColor', v)} />}
      {show('accentColor') && (
        <ColorField label="Accent color" value={values.accentColor ?? ''} onChange={(v) => set('accentColor', v)} />
      )}
      {show('successColor') && (
        <ColorField label="Success color" value={values.successColor ?? ''} onChange={(v) => set('successColor', v)} />
      )}
      {show('warningColor') && (
        <ColorField label="Warning color" value={values.warningColor ?? ''} onChange={(v) => set('warningColor', v)} />
      )}
      {show('errorColor') && (
        <ColorField label="Error color" value={values.errorColor ?? ''} onChange={(v) => set('errorColor', v)} />
      )}
      {show('infoColor') && <ColorField label="Info color" value={values.infoColor ?? ''} onChange={(v) => set('infoColor', v)} />}

      {show('fontFamily') && (
        <FontSelectField label="Font family" value={values.fontFamily ?? ''} onChange={(v) => set('fontFamily', v)} />
      )}
      {show('headingFontFamily') && (
        <FontSelectField
          label="Heading font family"
          value={values.headingFontFamily ?? ''}
          onChange={(v) => set('headingFontFamily', v)}
        />
      )}
      {show('fontSize') && (
        <label className="field">
          <span className="kicker">Font size</span>
          <input value={values.fontSize ?? ''} onChange={(e) => set('fontSize', e.target.value)} placeholder="16px" />
        </label>
      )}
      {show('fontSizeSmall') && (
        <label className="field">
          <span className="kicker">Small font size</span>
          <input
            value={values.fontSizeSmall ?? ''}
            onChange={(e) => set('fontSizeSmall', e.target.value)}
            placeholder="13px"
          />
        </label>
      )}
      {show('fontSizeLarge') && (
        <label className="field">
          <span className="kicker">Large font size</span>
          <input
            value={values.fontSizeLarge ?? ''}
            onChange={(e) => set('fontSizeLarge', e.target.value)}
            placeholder="28px"
          />
        </label>
      )}
      {show('borderRadius') && (
        <label className="field">
          <span className="kicker">Border radius</span>
          <input value={values.borderRadius ?? ''} onChange={(e) => set('borderRadius', e.target.value)} placeholder="12px" />
        </label>
      )}
      {show('opacity') && (
        <label className="field">
          <span className="kicker">Opacity</span>
          <input
            type="number"
            min={0}
            max={1}
            step={0.05}
            value={values.opacity ?? 1}
            onChange={(e) => set('opacity', Number(e.target.value))}
          />
        </label>
      )}
      {show('blur') && (
        <label className="field">
          <span className="kicker">Blur</span>
          <input value={values.blur ?? ''} onChange={(e) => set('blur', e.target.value)} placeholder="16px" />
        </label>
      )}
    </div>
  );
}

// A <select> over FONT_OPTIONS rather than free text -- a typo'd or
// unavailable font name used to fail silently (the browser just falls
// back to its default with no indication anything was wrong). Still
// shows the theme's actual value even if it isn't one of the curated
// options (an older custom value, or one hand-edited via JSON import)
// via a synthetic trailing option, rather than silently normalizing it
// to the first option the moment this renders.
function FontSelectField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  const isKnown = FONT_OPTIONS.some((f) => f.value === value);
  return (
    <label className="field">
      <span className="kicker">{label}</span>
      <select value={value} onChange={(e) => onChange(e.target.value)}>
        {!isKnown && value && <option value={value}>Custom ({value})</option>}
        {FONT_OPTIONS.map((f) => (
          <option key={f.value} value={f.value}>
            {f.label}
          </option>
        ))}
      </select>
    </label>
  );
}

// Parses any CSS color string this app actually stores (hex or rgb/rgba)
// into a plain 6-digit hex plus whatever alpha it carried, if any -- a
// native <input type="color"> only ever accepts/returns #rrggbb, so this
// is how its value is derived from (and reapplied on top of) values that
// may be rgba. Anything unparseable (a raw CSS keyword, an empty string)
// falls back to black rather than leaving the picker in an invalid state.
function toHexAndAlpha(value: string): { hex: string; alpha: number | null } {
  const rgbMatch = value.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)/i);
  if (rgbMatch) {
    const [, r, g, b, a] = rgbMatch;
    const hex = '#' + [r, g, b].map((n) => Number(n).toString(16).padStart(2, '0')).join('');
    return { hex, alpha: a !== undefined ? Number(a) : null };
  }
  if (/^#[0-9a-f]{6}$/i.test(value)) {
    return { hex: value, alpha: null };
  }
  if (/^#[0-9a-f]{3}$/i.test(value)) {
    const hex = '#' + value.slice(1).split('').map((c) => c + c).join('');
    return { hex, alpha: null };
  }
  return { hex: '#000000', alpha: null };
}

// Combines a hex color and an alpha (0-1) back into the string form the
// rest of the app expects -- a plain hex when fully opaque (matching how
// e.g. accent/text colors are already stored), otherwise rgba() so the
// alpha survives.
function composeColor(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  if (alpha >= 1) return hex;
  return `rgba(${r}, ${g}, ${b}, ${Math.round(alpha * 100) / 100})`;
}

interface Hsv {
  h: number; // 0-360
  s: number; // 0-100
  v: number; // 0-100
}

function hexToRgb(hex: string): [number, number, number] {
  return [parseInt(hex.slice(1, 3), 16), parseInt(hex.slice(3, 5), 16), parseInt(hex.slice(5, 7), 16)];
}

function rgbToHex(r: number, g: number, b: number): string {
  return (
    '#' +
    [r, g, b]
      .map((n) => Math.round(Math.min(255, Math.max(0, n))).toString(16).padStart(2, '0'))
      .join('')
  );
}

function rgbToHsv(r: number, g: number, b: number): Hsv {
  const rN = r / 255;
  const gN = g / 255;
  const bN = b / 255;
  const max = Math.max(rN, gN, bN);
  const min = Math.min(rN, gN, bN);
  const d = max - min;
  let h = 0;
  if (d !== 0) {
    if (max === rN) h = ((gN - bN) / d) % 6;
    else if (max === gN) h = (bN - rN) / d + 2;
    else h = (rN - gN) / d + 4;
    h *= 60;
    if (h < 0) h += 360;
  }
  return { h, s: max === 0 ? 0 : (d / max) * 100, v: max * 100 };
}

function hsvToRgb(h: number, s: number, v: number): [number, number, number] {
  const sN = s / 100;
  const vN = v / 100;
  const c = vN * sN;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = vN - c;
  let rgb: [number, number, number];
  if (h < 60) rgb = [c, x, 0];
  else if (h < 120) rgb = [x, c, 0];
  else if (h < 180) rgb = [0, c, x];
  else if (h < 240) rgb = [0, x, c];
  else if (h < 300) rgb = [x, 0, c];
  else rgb = [c, 0, x];
  return [(rgb[0] + m) * 255, (rgb[1] + m) * 255, (rgb[2] + m) * 255];
}

export function ColorField({
  label,
  value,
  onChange,
  helpText,
}: {
  // Omit for a compact, label-less variant -- reused by
  // GradientStopEditor, where each stop's own position control already
  // identifies the row and a repeated "Color" kicker per stop would just
  // be noise.
  label?: string;
  value: string;
  onChange: (v: string) => void;
  helpText?: string;
}) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const svRef = useRef<HTMLDivElement>(null);

  // hue/saturation/value/alpha live as their own state, not derived fresh
  // from `value` on every render -- round-tripping a fully desaturated or
  // fully dark color back through rgb->hsv loses its hue entirely (both
  // are 0 for e.g. pure black), which would otherwise snap the hue strip
  // back to red on every drag. Reseeded from the field's actual value
  // only when the popover opens, so it still reflects e.g. a value typed
  // directly into the text field while the popover was closed.
  const [hsv, setHsv] = useState<Hsv>({ h: 0, s: 0, v: 0 });
  const [alpha, setAlpha] = useState(1);

  useEffect(() => {
    if (!open) return;
    const { hex, alpha: a } = toHexAndAlpha(value || '#000000');
    const [r, g, b] = hexToRgb(hex);
    setHsv(rgbToHsv(r, g, b));
    setAlpha(a ?? 1);
    // Deliberately only on `open` -- see comment above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Closes on Escape or a click/mousedown anywhere outside the trigger
  // and the popover itself -- there's no separate "confirm" step (every
  // change already applies live, same as the raw text field next to it),
  // so this matches how a native color picker itself behaves rather than
  // requiring an explicit close button.
  useEffect(() => {
    if (!open) return;
    function handlePointerDown(e: MouseEvent) {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || popoverRef.current?.contains(target)) return;
      setOpen(false);
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  const [curR, curG, curB] = hsvToRgb(hsv.h, hsv.s, hsv.v);
  const curRgb = `${Math.round(curR)} ${Math.round(curG)} ${Math.round(curB)}`;

  function emit(nextHsv: Hsv, nextAlpha: number) {
    const [r, g, b] = hsvToRgb(nextHsv.h, nextHsv.s, nextHsv.v);
    onChange(composeColor(rgbToHex(r, g, b), nextAlpha));
  }

  function updateFromSvEvent(e: PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.min(Math.max(e.clientX - rect.left, 0), rect.width);
    const y = Math.min(Math.max(e.clientY - rect.top, 0), rect.height);
    const next = { ...hsv, s: (x / rect.width) * 100, v: 100 - (y / rect.height) * 100 };
    setHsv(next);
    emit(next, alpha);
  }

  return (
    // A plain div, not <label> -- a <label> with no `for` forwards a
    // click anywhere inside it (even on a non-control area) to its
    // first labelable descendant. With a button, a text input, and two
    // range sliders all nested in here, that meant every click inside
    // the popover -- the SV square, the space around the sliders, any
    // of it -- also fired a synthetic click on the swatch button,
    // instantly re-toggling `open` closed right after each pick.
    <div className="field">
      {label && <span className="kicker">{label}</span>}
      <div className="color-field-row">
        <button
          ref={triggerRef}
          type="button"
          className="color-swatch"
          onClick={() => setOpen((o) => !o)}
          aria-label={label ? `Pick ${label.toLowerCase()}` : 'Pick color'}
          title="Click to pick a color"
        >
          {/* Sits on top of the checkerboard on .color-swatch itself --
              an inline background on the button would replace the
              checkerboard outright instead of layering over it, hiding
              the one cue that shows a color is translucent. */}
          <span className="color-swatch-fill" style={{ background: value || 'transparent' }} />
        </button>
        <input value={value} onChange={(e) => onChange(e.target.value)} placeholder="#rrggbb or rgba(...)" />
        {open && (
          <div className="color-popover" ref={popoverRef}>
            {/* The saturation/value field -- every shade of the current
                hue, so picking a color happens here instead of only
                through a separate OS dialog. Background is the pure hue
                with a white-to-transparent and black-to-transparent
                overlay (the standard SV-square technique), so x = amount
                of white removed (saturation) and y = amount of black
                added (value). */}
            <div
              ref={svRef}
              className="color-popover-sv"
              style={{ backgroundColor: `hsl(${hsv.h}, 100%, 50%)` }}
              onPointerDown={(e) => {
                e.currentTarget.setPointerCapture(e.pointerId);
                updateFromSvEvent(e);
              }}
              onPointerMove={(e) => {
                if (e.buttons !== 1) return;
                updateFromSvEvent(e);
              }}
            >
              <span
                className="color-popover-sv-thumb"
                style={{ left: `${hsv.s}%`, top: `${100 - hsv.v}%`, background: `rgb(${curRgb})` }}
              />
            </div>

            <input
              type="range"
              className="color-popover-hue"
              min={0}
              max={360}
              value={hsv.h}
              onChange={(e) => {
                const next = { ...hsv, h: Number(e.target.value) };
                setHsv(next);
                emit(next, alpha);
              }}
              aria-label="Hue"
            />

            <div className="color-popover-bottom-row">
              {/* Shows the color actually being picked, alpha included,
                  distinct from both the SV thumb (mid-drag, easy to lose
                  track of against a busy gradient) and the field's own
                  trigger swatch (only updates once this popover closes
                  its live-onChange loop back around). */}
              <div className="color-popover-preview" title="Current selection">
                <span className="color-popover-preview-fill" style={{ background: `rgb(${curRgb} / ${alpha})` }} />
              </div>
              <div className="color-popover-alpha-row">
                <div className="color-popover-alpha-header">
                  <span>Transparency</span>
                  <span>{Math.round(alpha * 100)}%</span>
                </div>
                <input
                  type="range"
                  className="color-popover-alpha"
                  min={0}
                  max={1}
                  step={0.01}
                  value={alpha}
                  onChange={(e) => {
                    const next = Number(e.target.value);
                    setAlpha(next);
                    emit(hsv, next);
                  }}
                  aria-label="Transparency"
                  style={{ '--alpha-rgb': curRgb } as CSSProperties}
                />
              </div>
            </div>
          </div>
        )}
      </div>
      {helpText && <span className="field-help">{helpText}</span>}
    </div>
  );
}

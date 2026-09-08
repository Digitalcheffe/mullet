import type { ThemeTokens } from '../../shared/themes/tokens';
import './ThemeTokenFields.css';

type ThemeTokenKey = keyof ThemeTokens;

const ALL_FIELDS: ThemeTokenKey[] = [
  'background',
  'cardBackground',
  'cardBorder',
  'textColor',
  'accentColor',
  'fontFamily',
  'fontSize',
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
}

export default function ThemeTokenFields({ values, onChange, fields = ALL_FIELDS }: ThemeTokenFieldsProps) {
  function set<K extends ThemeTokenKey>(key: K, value: ThemeTokens[K]) {
    onChange({ ...values, [key]: value });
  }

  const show = (key: ThemeTokenKey) => fields.includes(key);

  return (
    <div className="theme-token-fields">
      {show('background') && (
        <label className="field">
          <span className="kicker">Background</span>
          <div className="background-field-row">
            <select
              value={values.background?.type ?? 'solid'}
              onChange={(e) =>
                set('background', {
                  type: e.target.value as ThemeTokens['background']['type'],
                  value: values.background?.value ?? '',
                })
              }
            >
              <option value="solid">Solid</option>
              <option value="gradient">Gradient</option>
              <option value="image">Image URL</option>
            </select>
            <input
              value={values.background?.value ?? ''}
              onChange={(e) => set('background', { type: values.background?.type ?? 'solid', value: e.target.value })}
              placeholder={
                values.background?.type === 'image'
                  ? 'https://...'
                  : values.background?.type === 'gradient'
                    ? 'linear-gradient(...)'
                    : '#0b0f14'
              }
            />
          </div>
        </label>
      )}

      {show('cardBackground') && (
        <ColorField label="Card background" value={values.cardBackground ?? ''} onChange={(v) => set('cardBackground', v)} />
      )}
      {show('cardBorder') && <ColorField label="Card border" value={values.cardBorder ?? ''} onChange={(v) => set('cardBorder', v)} />}
      {show('textColor') && <ColorField label="Text color" value={values.textColor ?? ''} onChange={(v) => set('textColor', v)} />}
      {show('accentColor') && (
        <ColorField label="Accent color" value={values.accentColor ?? ''} onChange={(v) => set('accentColor', v)} />
      )}

      {show('fontFamily') && (
        <label className="field">
          <span className="kicker">Font family</span>
          <input value={values.fontFamily ?? ''} onChange={(e) => set('fontFamily', e.target.value)} placeholder="system-ui, sans-serif" />
        </label>
      )}
      {show('fontSize') && (
        <label className="field">
          <span className="kicker">Font size</span>
          <input value={values.fontSize ?? ''} onChange={(e) => set('fontSize', e.target.value)} placeholder="16px" />
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

function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="field">
      <span className="kicker">{label}</span>
      <div className="color-field-row">
        <span className="color-swatch" style={{ background: value || 'transparent' }} />
        <input value={value} onChange={(e) => onChange(e.target.value)} placeholder="#rrggbb or rgba(...)" />
      </div>
    </label>
  );
}

import { useState, type FormEvent } from 'react';
import './ManifestForm.css';

export interface SetupField {
  key: string;
  label: string;
  type: 'text' | 'select' | 'multi-select' | 'toggle' | 'password' | 'number';
  required: boolean;
  default?: unknown;
  placeholder?: string;
  help_text?: string;
  options?: { value: string; label: string }[];
}

export interface PluginManifest {
  id: string;
  name: string;
  description: string;
  data_shapes: string[];
  setup_fields: SetupField[];
  auth_type: string;
  recommended_interval_seconds: number;
  min_interval_seconds: number;
}

export interface ManifestFormValues {
  instance_name: string;
  refresh_seconds: number;
  enabled: boolean;
  config: Record<string, unknown>;
}

interface ManifestFormProps {
  manifest: PluginManifest;
  initialValues: ManifestFormValues;
  submitLabel: string;
  onSubmit: (values: ManifestFormValues) => Promise<void>;
  onCancel: () => void;
}

// Renders a setup form from a plugin's manifest: an Instance Name +
// Refresh Interval + Enabled common to every plugin, plus one input per
// SetupField the manifest declares. Used for both adding a new instance
// and editing an existing one (pre-filled).
export default function ManifestForm({ manifest, initialValues, submitLabel, onSubmit, onCancel }: ManifestFormProps) {
  const [instanceName, setInstanceName] = useState(initialValues.instance_name);
  const [refreshSeconds, setRefreshSeconds] = useState(initialValues.refresh_seconds);
  const [enabled, setEnabled] = useState(initialValues.enabled);
  const [config, setConfig] = useState<Record<string, unknown>>(initialValues.config);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function setField(key: string, value: unknown) {
    setConfig((c) => ({ ...c, [key]: value }));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit({ instance_name: instanceName, refresh_seconds: refreshSeconds, enabled, config });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <label className="field">
        <span className="kicker">Instance name</span>
        <input value={instanceName} onChange={(e) => setInstanceName(e.target.value)} required autoFocus />
      </label>

      {manifest.setup_fields.map((field) => (
        <SetupFieldInput key={field.key} field={field} value={config[field.key]} onChange={(v) => setField(field.key, v)} />
      ))}

      <label className="field">
        <span className="kicker">Refresh interval (seconds)</span>
        <input
          type="number"
          min={manifest.min_interval_seconds || 1}
          value={refreshSeconds}
          onChange={(e) => setRefreshSeconds(Number(e.target.value))}
        />
      </label>

      <label className="toggle-row">
        <span>Enabled</span>
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
      </label>

      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : submitLabel}
        </button>
      </div>
    </form>
  );
}

function SetupFieldInput({
  field,
  value,
  onChange,
}: {
  field: SetupField;
  value: unknown;
  onChange: (v: unknown) => void;
}) {
  const help = field.help_text && <span className="field-help">{field.help_text}</span>;

  switch (field.type) {
    case 'toggle':
      return (
        <label className="toggle-row">
          <span>{field.label}</span>
          <input type="checkbox" checked={Boolean(value)} onChange={(e) => onChange(e.target.checked)} />
        </label>
      );

    case 'select':
      return (
        <label className="field">
          <span className="kicker">{field.label}</span>
          <select value={(value as string) ?? ''} onChange={(e) => onChange(e.target.value)} required={field.required}>
            <option value="" disabled>
              Select…
            </option>
            {field.options?.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          {help}
        </label>
      );

    case 'multi-select':
      return (
        <label className="field">
          <span className="kicker">{field.label}</span>
          <select
            multiple
            value={(value as string[]) ?? []}
            onChange={(e) => onChange(Array.from(e.target.selectedOptions).map((o) => o.value))}
          >
            {field.options?.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          {help}
        </label>
      );

    case 'number':
      return (
        <label className="field">
          <span className="kicker">{field.label}</span>
          <input
            type="number"
            value={(value as number) ?? ''}
            placeholder={field.placeholder}
            onChange={(e) => onChange(Number(e.target.value))}
            required={field.required}
          />
          {help}
        </label>
      );

    case 'password':
      return (
        <label className="field">
          <span className="kicker">{field.label}</span>
          <input
            type="password"
            value={(value as string) ?? ''}
            placeholder={field.placeholder}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          />
          {help}
        </label>
      );

    case 'text':
    default:
      return (
        <label className="field">
          <span className="kicker">{field.label}</span>
          <input
            value={(value as string) ?? ''}
            placeholder={field.placeholder}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          />
          {help}
        </label>
      );
  }
}

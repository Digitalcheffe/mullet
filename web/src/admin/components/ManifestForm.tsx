import { useEffect, useState, type FormEvent } from 'react';
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
  // dynamic marks a field whose real options only exist once this
  // instance has been authorized (e.g. "which of your Outlook
  // calendars") -- options is always empty in the manifest for one of
  // these; ManifestForm fetches the live list itself, when it can.
  dynamic?: boolean;
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

interface DiscoveredOption {
  value: string;
  label: string;
}

interface ManifestFormProps {
  manifest: PluginManifest;
  initialValues: ManifestFormValues;
  submitLabel: string;
  onSubmit: (values: ManifestFormValues) => Promise<void>;
  onCancel: () => void;
  // instanceId/oauthAuthorized are only present in 'edit' mode. A
  // Dynamic field's real options can't be fetched before the instance
  // exists, since discovery is scoped to one specific instance's own
  // config -- and for an OAuth2 plugin specifically, not before it's
  // been authorized either, since discovery there needs a live token.
  // A non-OAuth2 plugin (e.g. home-assistant's static token) has no such
  // extra step: oauthAuthorized is simply ignored for it. apiFetch is
  // the admin's own authenticated fetch (see useApiFetch), needed to
  // call the discover endpoint.
  instanceId?: number;
  oauthAuthorized?: boolean;
  apiFetch?: (path: string, init?: RequestInit) => Promise<Response>;
}

// Renders a setup form from a plugin's manifest: an Instance Name +
// Refresh Interval + Enabled common to every plugin, plus one input per
// SetupField the manifest declares. Used for both adding a new instance
// and editing an existing one (pre-filled).
export default function ManifestForm({
  manifest,
  initialValues,
  submitLabel,
  onSubmit,
  onCancel,
  instanceId,
  oauthAuthorized,
  apiFetch,
}: ManifestFormProps) {
  const [instanceName, setInstanceName] = useState(initialValues.instance_name);
  const [refreshSeconds, setRefreshSeconds] = useState(initialValues.refresh_seconds);
  const [enabled, setEnabled] = useState(initialValues.enabled);
  const [config, setConfig] = useState<Record<string, unknown>>(initialValues.config);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [discovered, setDiscovered] = useState<Record<string, DiscoveredOption[]>>({});
  const [discovering, setDiscovering] = useState<Record<string, boolean>>({});

  // An OAuth2 plugin needs the extra authorize step before discovery can
  // work; any other AuthType's instance config already has everything
  // Discover needs the moment the instance exists.
  const requiresAuthorize = manifest.auth_type === 'oauth2';
  const canDiscover = Boolean(instanceId && (!requiresAuthorize || oauthAuthorized) && apiFetch);

  useEffect(() => {
    if (!canDiscover) return;
    for (const field of manifest.setup_fields) {
      if (!field.dynamic) continue;
      setDiscovering((d) => ({ ...d, [field.key]: true }));
      apiFetch!(`/api/admin/plugins/instances/${instanceId}/discover?field=${encodeURIComponent(field.key)}`)
        .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`status ${res.status}`))))
        .then((options: DiscoveredOption[]) => setDiscovered((d) => ({ ...d, [field.key]: options })))
        .catch(() => setDiscovered((d) => ({ ...d, [field.key]: [] })))
        .finally(() => setDiscovering((d) => ({ ...d, [field.key]: false })));
    }
    // Re-run only when the instance/authorization identity actually
    // changes -- manifest.setup_fields is stable for a given plugin type.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canDiscover, instanceId]);

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
        <SetupFieldInput
          key={field.key}
          field={field}
          value={config[field.key]}
          onChange={(v) => setField(field.key, v)}
          discoveredOptions={discovered[field.key]}
          discovering={Boolean(discovering[field.key])}
          canDiscover={canDiscover}
        />
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
  discoveredOptions,
  discovering,
  canDiscover,
}: {
  field: SetupField;
  value: unknown;
  onChange: (v: unknown) => void;
  discoveredOptions?: { value: string; label: string }[];
  discovering?: boolean;
  canDiscover?: boolean;
}) {
  // A Dynamic field's manifest-declared options are always empty --
  // its real choices come from discovery instead, which can only run
  // once this instance exists and has been authorized. Until then (add
  // mode, or an as-yet-unauthorized instance), render a plain status
  // note rather than a non-functional empty select.
  if (field.dynamic) {
    if (discovering) {
      return (
        <div className="field">
          <span className="kicker">{field.label}</span>
          <p className="field-help">Loading options…</p>
        </div>
      );
    }
    if (!canDiscover) {
      return (
        <div className="field">
          <span className="kicker">{field.label}</span>
          <p className="field-help">{field.help_text || 'Save this instance first (and authorize it, if it needs that), then edit it to choose specific values.'}</p>
        </div>
      );
    }
    // help_text's job was guiding the admin through "authorize first" --
    // moot now that real options are showing, so it's dropped here
    // rather than lingering next to a working multi-select.
    field = { ...field, type: 'multi-select', options: discoveredOptions ?? [], help_text: undefined };
    if ((discoveredOptions ?? []).length === 0) {
      return (
        <div className="field">
          <span className="kicker">{field.label}</span>
          <p className="field-help">No options found.</p>
        </div>
      );
    }
  }

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

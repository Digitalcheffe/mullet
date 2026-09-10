import { useCallback, useEffect, useState } from 'react';
import ManifestForm, { type ManifestFormValues, type PluginManifest } from '../components/ManifestForm';
import { useApiFetch } from '../auth/useApiFetch';
import './PluginsPage.css';

interface PluginInstance {
  id: number;
  plugin_id: string;
  instance_name: string;
  config: Record<string, unknown>;
  refresh_seconds: number;
  enabled: boolean;
  status: 'synced' | 'retrying' | 'pending' | 'disabled';
  last_error?: string;
  oauth_authorized: boolean;
}

const statusLabel: Record<PluginInstance['status'], string> = {
  synced: 'Synced',
  retrying: 'Retrying',
  pending: 'Pending',
  disabled: 'Disabled',
};

type Panel =
  | { mode: 'add'; manifest: PluginManifest }
  | { mode: 'edit'; manifest: PluginManifest; instance: PluginInstance }
  | null;

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

export default function PluginsPage() {
  const apiFetch = useApiFetch();
  const [manifests, setManifests] = useState<PluginManifest[]>([]);
  const [instances, setInstances] = useState<PluginInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [panel, setPanel] = useState<Panel>(null);
  const [oauthNotice, setOauthNotice] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);

  const load = useCallback(() => {
    return Promise.all([
      apiFetch('/api/admin/plugins').then((r) => r.json()),
      apiFetch('/api/admin/plugins/instances').then((r) => r.json()),
    ]).then(([m, i]) => {
      setManifests(m);
      setInstances(i);
      setLoading(false);
    });
  }, [apiFetch]);

  useEffect(() => {
    load();
  }, [load]);

  // The OAuth callback (internal/api/oauth_handlers.go) redirects the
  // browser back here with ?oauth=success or ?oauth=error&message=... --
  // it can't hand the result back any other way, since granting consent
  // is a real full-page navigation away to the provider and back (which,
  // notably, also means the in-memory admin session is gone and the
  // login gate reappears; the token itself was already saved
  // server-side before this redirect, so nothing is lost, just the
  // browser session). Read it once on mount and scrub it from the URL so
  // a refresh doesn't re-show the same notice.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const oauth = params.get('oauth');
    if (oauth === 'success') {
      setOauthNotice({ kind: 'success', text: 'Plugin authorized.' });
    } else if (oauth === 'error') {
      setOauthNotice({ kind: 'error', text: params.get('message') || 'Authorization failed.' });
    }
    if (oauth) {
      window.history.replaceState(null, '', window.location.pathname);
    }
  }, []);

  async function handleAuthorize(instance: PluginInstance) {
    const res = await apiFetch(`/api/admin/plugins/instances/${instance.id}/oauth/authorize`);
    if (!res.ok) {
      setOauthNotice({ kind: 'error', text: await readErrorMessage(res, 'Could not start authorization') });
      return;
    }
    const { authorize_url }: { authorize_url: string } = await res.json();
    window.location.href = authorize_url;
  }

  async function handleDeauthorize(instance: PluginInstance) {
    if (!confirm(`Revoke authorization for "${instance.instance_name}"?`)) return;
    await apiFetch(`/api/admin/plugins/instances/${instance.id}/oauth`, { method: 'DELETE' });
    load();
  }

  async function submitInstance(pluginID: string, values: ManifestFormValues, url: string, method: 'POST' | 'PUT') {
    const res = await apiFetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        plugin_id: pluginID,
        instance_name: values.instance_name,
        refresh_seconds: values.refresh_seconds,
        enabled: values.enabled,
        config: values.config,
      }),
    });
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Save failed'));
    }
    setPanel(null);
    await load();

    // A freshly created OAuth2 instance can't fetch anything until it's
    // authorized -- send the admin straight into the provider's consent
    // screen instead of leaving them to find a separate "Authorize"
    // button in the list this just refreshed into.
    if (method === 'POST' && manifests.find((m) => m.id === pluginID)?.auth_type === 'oauth2') {
      const created: PluginInstance = await res.json();
      await handleAuthorize(created);
    }
  }

  async function handleDelete(instance: PluginInstance) {
    if (!confirm(`Remove "${instance.instance_name}"?`)) return;
    await apiFetch(`/api/admin/plugins/instances/${instance.id}`, { method: 'DELETE' });
    load();
  }

  async function handleToggleEnabled(instance: PluginInstance) {
    await apiFetch(`/api/admin/plugins/instances/${instance.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        plugin_id: instance.plugin_id,
        instance_name: instance.instance_name,
        refresh_seconds: instance.refresh_seconds,
        enabled: !instance.enabled,
        config: instance.config,
      }),
    });
    load();
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  return (
    <div className="plugins-page">
      <h1>Data Plugins</h1>

      {oauthNotice && (
        <p className={`oauth-notice oauth-notice-${oauthNotice.kind}`} role="alert">
          {oauthNotice.text}
        </p>
      )}

      <section>
        <h2>Configured Instances</h2>
        {instances.length === 0 ? (
          <div className="empty-state">No plugin instances configured yet.</div>
        ) : (
          <div className="instance-list">
            {instances.map((inst) => (
              <div className="instance-row" key={inst.id}>
                <div className="instance-info">
                  <div className="instance-name">{inst.instance_name}</div>
                  <div className="instance-detail">
                    {inst.plugin_id} &middot; every {inst.refresh_seconds}s
                  </div>
                  {inst.last_error && <div className="instance-error">{inst.last_error}</div>}
                </div>
                <span className={`status-pill ${inst.status}`}>
                  <span className="status-dot" />
                  {statusLabel[inst.status]}
                </span>
                <label className="mini-toggle" title="Enabled">
                  <input type="checkbox" checked={inst.enabled} onChange={() => handleToggleEnabled(inst)} />
                </label>
                {manifests.find((m) => m.id === inst.plugin_id)?.auth_type === 'oauth2' &&
                  (inst.oauth_authorized ? (
                    <>
                      <span className="oauth-authorized-pill">Authorized</span>
                      <button className="btn-secondary" onClick={() => handleAuthorize(inst)}>
                        Re-authorize
                      </button>
                      <button className="btn-secondary" onClick={() => handleDeauthorize(inst)}>
                        Revoke
                      </button>
                    </>
                  ) : (
                    <button className="btn-primary" onClick={() => handleAuthorize(inst)}>
                      Authorize
                    </button>
                  ))}
                <button
                  className="btn-secondary"
                  onClick={() => {
                    const manifest = manifests.find((m) => m.id === inst.plugin_id);
                    if (manifest) setPanel({ mode: 'edit', manifest, instance: inst });
                  }}
                >
                  Edit
                </button>
                <button className="btn-danger" onClick={() => handleDelete(inst)}>
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      <section>
        <h2>Available Plugins</h2>
        <div className="available-plugins">
          {manifests.map((m) => (
            <div className="available-plugin-card" key={m.id}>
              <div>
                <div className="plugin-card-name">{m.name}</div>
                <div className="plugin-card-desc">{m.description}</div>
              </div>
              <button className="btn-secondary" onClick={() => setPanel({ mode: 'add', manifest: m })}>
                + Add
              </button>
            </div>
          ))}
        </div>
      </section>

      {panel && (
        <div className="modal-scrim">
          <div className="modal-panel">
            <h2>{panel.mode === 'add' ? `Add ${panel.manifest.name}` : `Edit ${panel.instance.instance_name}`}</h2>
            <ManifestForm
              manifest={panel.manifest}
              initialValues={
                panel.mode === 'add'
                  ? {
                      instance_name: '',
                      refresh_seconds: panel.manifest.recommended_interval_seconds,
                      enabled: true,
                      config: {},
                    }
                  : {
                      instance_name: panel.instance.instance_name,
                      refresh_seconds: panel.instance.refresh_seconds,
                      enabled: panel.instance.enabled,
                      config: panel.instance.config,
                    }
              }
              submitLabel={panel.mode === 'add' ? 'Add plugin' : 'Save changes'}
              onSubmit={(values) =>
                panel.mode === 'add'
                  ? submitInstance(panel.manifest.id, values, '/api/admin/plugins/instances', 'POST')
                  : submitInstance(panel.manifest.id, values, `/api/admin/plugins/instances/${panel.instance.id}`, 'PUT')
              }
              onCancel={() => setPanel(null)}
              instanceId={panel.mode === 'edit' ? panel.instance.id : undefined}
              oauthAuthorized={panel.mode === 'edit' ? panel.instance.oauth_authorized : undefined}
              apiFetch={apiFetch}
            />
          </div>
        </div>
      )}
    </div>
  );
}

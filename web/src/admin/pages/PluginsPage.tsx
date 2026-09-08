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

      {panel && (
        <div className="modal-scrim" onClick={() => setPanel(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
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
            />
          </div>
        </div>
      )}
    </div>
  );
}

import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './ClientsPage.css';

type ClientStatus = 'pending' | 'approved' | 'rejected';
type OfflineMode = 'offline_screen' | 'screen_off' | 'last_screenshot';

interface Client {
  id: number;
  client_id: string;
  name: string;
  display_id?: number;
  status: ClientStatus;
  online: boolean;
  last_seen_at?: string;
  offline_mode: OfflineMode;
  platform?: string;
  app_version?: string;
  ip_address?: string;
}

interface Display {
  id: number;
  name: string;
  slug: string;
}

const statusLabel: Record<ClientStatus, string> = { pending: 'Pending', approved: 'Approved', rejected: 'Rejected' };
const offlineModeLabel: Record<OfflineMode, string> = {
  offline_screen: 'Show offline screen',
  screen_off: 'Turn screen off (HDMI-CEC)',
  last_screenshot: 'Freeze on last frame',
};

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

function formatLastSeen(iso?: string): string {
  if (!iso) return 'Never';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const seconds = Math.max(0, (Date.now() - d.getTime()) / 1000);
  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return d.toLocaleDateString();
}

// escapeHtml/buildOfflineScreenHTML mirror internal/api/client_handlers.go's
// own defaultOfflineScreenHTML -- same visual language (dark kiosk, live
// clock, offline badge), just parameterized by an admin-editable title
// and message instead of the server's fixed defaults. A leading HTML
// comment carries the title/message back out in machine-readable form
// (see parseOfflineScreenHTML) so re-opening the editor doesn't need to
// parse the rendered markup itself.
function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function buildOfflineScreenHTML(title: string, message: string): string {
  return `<!-- mullet-offline-screen title=${JSON.stringify(title)} message=${JSON.stringify(message)} -->
<!doctype html>
<html><head><meta charset="utf-8"><style>
html,body{margin:0;height:100%;background:#0b0f14;color:#e6e9ee;font-family:system-ui,sans-serif;
  display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px}
.offline-time{font-size:4em;font-weight:700}
.offline-date{opacity:0.7}
.offline-name{font-size:1.4em;font-weight:600;margin-top:16px}
.offline-badge{position:fixed;top:16px;right:20px;font-size:0.9em;opacity:0.6}
.offline-note{opacity:0.5;font-size:0.9em;margin-top:24px}
</style></head>
<body>
<div class="offline-badge">&#9679; offline</div>
<div class="offline-time" id="t"></div>
<div class="offline-date" id="d"></div>
<div class="offline-name">${escapeHtml(title)}</div>
<div class="offline-note">${escapeHtml(message)}</div>
<script>
function tick(){
  var n=new Date();
  document.getElementById('t').textContent=n.toLocaleTimeString([], {hour:'numeric',minute:'2-digit'});
  document.getElementById('d').textContent=n.toLocaleDateString([], {weekday:'long',month:'long',day:'numeric'});
}
tick(); setInterval(tick, 1000);
</script>
</body></html>`;
}

function parseOfflineScreenHTML(html: string): { title: string; message: string } | null {
  const m = html.match(/^<!-- mullet-offline-screen title=(".*?") message=(".*?") -->/);
  if (!m) return null;
  try {
    return { title: JSON.parse(m[1]), message: JSON.parse(m[2]) };
  } catch {
    return null;
  }
}

type ApprovePanel = { client: Client } | null;
type EditPanel = { client: Client } | null;
type OfflinePanel = { display: Display } | null;

export default function ClientsPage() {
  const apiFetch = useApiFetch();
  const [clients, setClients] = useState<Client[]>([]);
  const [displays, setDisplays] = useState<Display[]>([]);
  const [loading, setLoading] = useState(true);
  const [approvePanel, setApprovePanel] = useState<ApprovePanel>(null);
  const [editPanel, setEditPanel] = useState<EditPanel>(null);
  const [offlinePanel, setOfflinePanel] = useState<OfflinePanel>(null);

  const load = useCallback(() => {
    return Promise.all([
      apiFetch('/api/admin/clients').then((r) => r.json()),
      apiFetch('/api/admin/displays').then((r) => r.json()),
    ]).then(([c, d]) => {
      setClients(c);
      setDisplays(d);
      setLoading(false);
    });
  }, [apiFetch]);

  useEffect(() => {
    load();
  }, [load]);

  function displayName(id?: number): string {
    if (id == null) return 'Unassigned';
    return displays.find((d) => d.id === id)?.name ?? `Display #${id}`;
  }

  async function handleApprove(client: Client, displayId: number) {
    const res = await apiFetch(`/api/admin/clients/${client.id}/approve`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_id: displayId }),
    });
    if (!res.ok) throw new Error(await readErrorMessage(res, 'Approve failed'));
    setApprovePanel(null);
    await load();
  }

  async function updateClient(
    client: Client,
    patch: { name?: string; display_id?: number | null; status?: ClientStatus; offline_mode?: OfflineMode },
  ) {
    const res = await apiFetch(`/api/admin/clients/${client.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: patch.name ?? client.name,
        display_id: 'display_id' in patch ? patch.display_id ?? null : (client.display_id ?? null),
        status: patch.status ?? client.status,
        offline_mode: patch.offline_mode ?? client.offline_mode,
      }),
    });
    if (!res.ok) throw new Error(await readErrorMessage(res, 'Save failed'));
    await load();
  }

  async function handleReject(client: Client) {
    await updateClient(client, { status: 'rejected' });
  }

  async function handleRevoke(client: Client) {
    if (!confirm(`Revoke "${client.name}"? It goes back to pending and stops showing its assigned display.`)) return;
    await updateClient(client, { status: 'pending', display_id: null });
  }

  async function handleDelete(client: Client) {
    if (!confirm(`Remove "${client.name}"? It will need to pair again from scratch to reconnect.`)) return;
    await apiFetch(`/api/admin/clients/${client.id}`, { method: 'DELETE' });
    await load();
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  const pending = clients.filter((c) => c.status === 'pending');
  const others = clients.filter((c) => c.status !== 'pending');

  return (
    <div className="clients-page">
      <h1>Clients</h1>

      {pending.length > 0 && (
        <section>
          <div className="section-header">
            <h2>Waiting for Approval</h2>
          </div>
          <div className="client-list">
            {pending.map((c) => (
              <div className="client-row pending-row" key={c.id}>
                <div className="client-info">
                  <div className="client-name">{c.name}</div>
                  <div className="pairing-code">{c.client_id}</div>
                  <div className="client-detail">
                    {c.platform ?? 'unknown platform'}
                    {c.ip_address && <span className="client-ip">{c.ip_address}</span>}
                  </div>
                </div>
                <button className="btn-primary" onClick={() => setApprovePanel({ client: c })}>
                  Approve
                </button>
                <button className="btn-secondary" onClick={() => handleReject(c)}>
                  Reject
                </button>
                <button className="btn-danger" onClick={() => handleDelete(c)}>
                  Delete
                </button>
              </div>
            ))}
          </div>
        </section>
      )}

      <section>
        <div className="section-header">
          <h2>All Clients</h2>
        </div>
        {others.length === 0 ? (
          <div className="empty-state">
            No clients yet. Install a client app on an endpoint device and it will show up here once it registers.
          </div>
        ) : (
          <div className="client-list">
            {others.map((c) => (
              <div className="client-row" key={c.id}>
                <span className={`status-pill ${c.status}`}>
                  <span className="status-dot" />
                  {statusLabel[c.status]}
                </span>
                <div className="client-info">
                  <div className="client-name">{c.name}</div>
                  <div className="client-detail">
                    {displayName(c.display_id)} &middot; {c.platform ?? 'unknown platform'}
                    {c.app_version ? ` v${c.app_version}` : ''} &middot; last seen {formatLastSeen(c.last_seen_at)}
                    {c.ip_address && <span className="client-ip">{c.ip_address}</span>}
                    {c.status === 'approved' && <span className={`online-dot${c.online ? ' online' : ''}`} />}
                  </div>
                </div>
                {c.status === 'approved' && (
                  <button className="btn-secondary" onClick={() => setEditPanel({ client: c })}>
                    Edit
                  </button>
                )}
                {c.status === 'approved' && (
                  <button className="btn-secondary" onClick={() => handleRevoke(c)}>
                    Revoke
                  </button>
                )}
                {c.status === 'rejected' && (
                  <button className="btn-primary" onClick={() => setApprovePanel({ client: c })}>
                    Approve
                  </button>
                )}
                <button className="btn-danger" onClick={() => handleDelete(c)}>
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      <section>
        <div className="section-header">
          <h2>Offline Screens</h2>
        </div>
        <p className="section-note">
          What a client shows when it can't reach the server. Leave a display unconfigured to use the generated
          default (a live clock and the display's name).
        </p>
        {displays.length === 0 ? (
          <div className="empty-state">No displays yet -- add one on the Displays page first.</div>
        ) : (
          <div className="client-list">
            {displays.map((d) => (
              <div className="client-row" key={d.id}>
                <div className="client-info">
                  <div className="client-name">{d.name}</div>
                  <div className="client-detail">/display/{d.slug}</div>
                </div>
                <button className="btn-secondary" onClick={() => setOfflinePanel({ display: d })}>
                  Design offline screen
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {approvePanel && (
        <div className="modal-scrim" onClick={() => setApprovePanel(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2>Approve &ldquo;{approvePanel.client.name}&rdquo;</h2>
            <ApproveForm displays={displays} onSubmit={(id) => handleApprove(approvePanel.client, id)} onCancel={() => setApprovePanel(null)} />
          </div>
        </div>
      )}

      {editPanel && (
        <div className="modal-scrim" onClick={() => setEditPanel(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2>Edit &ldquo;{editPanel.client.name}&rdquo;</h2>
            <EditClientForm
              client={editPanel.client}
              displays={displays}
              onSubmit={(values) => updateClient(editPanel.client, values).then(() => setEditPanel(null))}
              onCancel={() => setEditPanel(null)}
            />
          </div>
        </div>
      )}

      {offlinePanel && (
        <div className="modal-scrim" onClick={() => setOfflinePanel(null)}>
          <div className="modal-panel modal-panel-wide" onClick={(e) => e.stopPropagation()}>
            <h2>Offline Screen for &ldquo;{offlinePanel.display.name}&rdquo;</h2>
            <OfflineScreenForm apiFetch={apiFetch} display={offlinePanel.display} onDone={() => setOfflinePanel(null)} />
          </div>
        </div>
      )}
    </div>
  );
}

interface ApproveFormProps {
  displays: Display[];
  onSubmit: (displayId: number) => Promise<void>;
  onCancel: () => void;
}

function ApproveForm({ displays, onSubmit, onCancel }: ApproveFormProps) {
  const [displayId, setDisplayId] = useState<number | ''>(displays[0]?.id ?? '');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (displayId === '') return;
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(displayId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Approve failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (displays.length === 0) {
    return (
      <p className="form-error" role="alert">
        No displays exist yet -- add one on the Displays page before approving a client.
      </p>
    );
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <label className="field">
        <span className="kicker">Assign to display</span>
        <select value={displayId} onChange={(e) => setDisplayId(Number(e.target.value))} required autoFocus>
          {displays.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>
      </label>
      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Approving…' : 'Approve'}
        </button>
      </div>
    </form>
  );
}

interface EditClientFormProps {
  client: Client;
  displays: Display[];
  onSubmit: (values: { name: string; display_id: number | null; offline_mode: OfflineMode }) => Promise<void>;
  onCancel: () => void;
}

function EditClientForm({ client, displays, onSubmit, onCancel }: EditClientFormProps) {
  const [name, setName] = useState(client.name);
  const [displayId, setDisplayId] = useState<number | ''>(client.display_id ?? '');
  const [offlineMode, setOfflineMode] = useState<OfflineMode>(client.offline_mode);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit({ name, display_id: displayId === '' ? null : displayId, offline_mode: offlineMode });
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
        <span className="kicker">Name</span>
        <input value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
      </label>
      <label className="field">
        <span className="kicker">Display</span>
        <select value={displayId} onChange={(e) => setDisplayId(e.target.value === '' ? '' : Number(e.target.value))}>
          <option value="">Unassigned</option>
          {displays.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>
      </label>
      <label className="field">
        <span className="kicker">When offline</span>
        <select value={offlineMode} onChange={(e) => setOfflineMode(e.target.value as OfflineMode)}>
          {(Object.keys(offlineModeLabel) as OfflineMode[]).map((m) => (
            <option key={m} value={m}>
              {offlineModeLabel[m]}
            </option>
          ))}
        </select>
        <span className="field-help">
          {offlineModeLabel.screen_off === offlineModeLabel[offlineMode] && 'Requires HDMI-CEC support (Raspberry Pi only).'}
        </span>
      </label>
      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  );
}

interface OfflineScreenFormProps {
  apiFetch: ReturnType<typeof useApiFetch>;
  display: Display;
  onDone: () => void;
}

function OfflineScreenForm({ apiFetch, display, onDone }: OfflineScreenFormProps) {
  const [loading, setLoading] = useState(true);
  const [hasCustom, setHasCustom] = useState(false);
  const [title, setTitle] = useState(display.name);
  const [message, setMessage] = useState('Waiting for server connection');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    apiFetch(`/api/admin/displays/${display.id}/offline-screen`)
      .then((r) => r.json())
      .then((body: { html: string | null }) => {
        if (body.html) {
          setHasCustom(true);
          const parsed = parseOfflineScreenHTML(body.html);
          if (parsed) {
            setTitle(parsed.title);
            setMessage(parsed.message);
          }
        }
        setLoading(false);
      });
    // Only ever runs for the display this panel was opened for.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [display.id]);

  async function save(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await apiFetch(`/api/admin/displays/${display.id}/offline-screen`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ html: buildOfflineScreenHTML(title, message) }),
      });
      if (!res.ok) throw new Error(await readErrorMessage(res, 'Save failed'));
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function resetToDefault() {
    setSubmitting(true);
    try {
      const res = await apiFetch(`/api/admin/displays/${display.id}/offline-screen`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ html: null }),
      });
      if (!res.ok) throw new Error(await readErrorMessage(res, 'Reset failed'));
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  return (
    <form className="manifest-form offline-screen-form" onSubmit={save}>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="offline-screen-layout">
        <div className="offline-screen-fields">
          <label className="field">
            <span className="kicker">Title</span>
            <input value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus />
          </label>
          <label className="field">
            <span className="kicker">Message</span>
            <input value={message} onChange={(e) => setMessage(e.target.value)} required />
            <span className="field-help">Shown below a live clock, with a small "offline" indicator.</span>
          </label>
          {!hasCustom && <span className="field-help">Currently using the generated default -- saving switches to this custom one.</span>}
        </div>
        <iframe
          className="offline-screen-preview"
          title="Offline screen preview"
          sandbox="allow-scripts"
          srcDoc={buildOfflineScreenHTML(title, message)}
        />
      </div>
      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={resetToDefault} disabled={submitting}>
          Reset to default
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : 'Save'}
        </button>
      </div>
    </form>
  );
}

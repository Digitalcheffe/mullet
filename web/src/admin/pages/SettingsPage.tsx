import { useEffect, useState, type FormEvent } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './SettingsPage.css';

interface Settings {
  server_name: string;
  port: string;
  db_path: string;
}

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error';

export default function SettingsPage() {
  const apiFetch = useApiFetch();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [serverName, setServerName] = useState('');
  const [status, setStatus] = useState<SaveStatus>('idle');

  useEffect(() => {
    let cancelled = false;

    apiFetch('/api/admin/settings')
      .then((res) => res.json())
      .then((body: Settings) => {
        if (cancelled) return;
        setSettings(body);
        setServerName(body.server_name);
      });

    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_name: serverName }),
      });
      if (!res.ok) throw new Error('save failed');
      setStatus('saved');
    } catch {
      setStatus('error');
    }
  }

  if (!settings) {
    return <p>Loading…</p>;
  }

  return (
    <div className="settings-page">
      <h1>Settings</h1>

      <div className="settings-card">
        <form className="settings-form" onSubmit={handleSubmit}>
          <label className="field">
            <span className="kicker">Server name</span>
            <input
              value={serverName}
              onChange={(e) => {
                setServerName(e.target.value);
                setStatus('idle');
              }}
              required
            />
          </label>
          <div className="settings-form-actions">
            <button type="submit" className="btn-primary" disabled={status === 'saving'}>
              {status === 'saving' ? 'Saving…' : 'Save'}
            </button>
            {status === 'saved' && <span className="save-note ok">Saved.</span>}
            {status === 'error' && (
              <span className="save-note error" role="alert">
                Failed to save.
              </span>
            )}
          </div>
        </form>

        <div className="settings-readonly">
          <div className="field">
            <span className="kicker">Port</span>
            <div className="readonly-value">{settings.port}</div>
          </div>
          <div className="field">
            <span className="kicker">Database path</span>
            <div className="readonly-value">{settings.db_path}</div>
          </div>
        </div>
      </div>
    </div>
  );
}

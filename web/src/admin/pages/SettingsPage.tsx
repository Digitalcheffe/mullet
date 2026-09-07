import { useEffect, useState, type FormEvent } from 'react';
import { useApiFetch } from '../auth/useApiFetch';

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
    <div>
      <h2>Settings</h2>
      <form onSubmit={handleSubmit}>
        <label>
          Server name
          <input
            value={serverName}
            onChange={(e) => {
              setServerName(e.target.value);
              setStatus('idle');
            }}
            required
          />
        </label>
        <button type="submit" disabled={status === 'saving'}>
          {status === 'saving' ? 'Saving…' : 'Save'}
        </button>
        {status === 'saved' && <span> Saved.</span>}
        {status === 'error' && <span role="alert"> Failed to save.</span>}
      </form>
      <dl>
        <dt>Port</dt>
        <dd>{settings.port}</dd>
        <dt>Database path</dt>
        <dd>{settings.db_path}</dd>
      </dl>
    </div>
  );
}

import { useEffect, useState, type FormEvent } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './SettingsPage.css';

interface Settings {
  server_name: string;
  port: string;
  db_path: string;
}

interface SMTPConfig {
  host: string;
  port: number;
  username: string;
  from_address: string;
  tls_mode: string;
  has_password: boolean;
}

interface WhoAmI {
  user_id: number;
  username: string;
  email: string | null;
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
        <h2>Server</h2>
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

      <AccountSection apiFetch={apiFetch} />
      <SMTPSection apiFetch={apiFetch} />
    </div>
  );
}

function AccountSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [email, setEmail] = useState('');
  const [loaded, setLoaded] = useState(false);
  const [status, setStatus] = useState<SaveStatus>('idle');

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/me')
      .then((res) => res.json())
      .then((body: WhoAmI) => {
        if (cancelled) return;
        setEmail(body.email ?? '');
        setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/account/email', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error('save failed');
      setStatus('saved');
    } catch {
      setStatus('error');
    }
  }

  if (!loaded) {
    return null;
  }

  return (
    <div className="settings-card">
      <h2>Account</h2>
      <p className="settings-form-help">
        Used to receive password reset links and as the default recipient for SMTP test emails.
      </p>
      <form className="settings-form" onSubmit={handleSubmit}>
        <label className="field">
          <span className="kicker">Email</span>
          <input
            type="email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              setStatus('idle');
            }}
            placeholder="you@example.com"
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
    </div>
  );
}

function SMTPSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [loaded, setLoaded] = useState(false);
  const [hasPassword, setHasPassword] = useState(false);
  const [host, setHost] = useState('');
  const [port, setPort] = useState('587');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [fromAddress, setFromAddress] = useState('');
  const [tlsMode, setTlsMode] = useState('starttls');
  const [status, setStatus] = useState<SaveStatus>('idle');

  const [testTo, setTestTo] = useState('');
  const [testStatus, setTestStatus] = useState<SaveStatus>('idle');
  const [testError, setTestError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/settings/smtp')
      .then((res) => res.json())
      .then((body: SMTPConfig) => {
        if (cancelled) return;
        setHost(body.host);
        setPort(body.port ? String(body.port) : '587');
        setUsername(body.username);
        setFromAddress(body.from_address);
        setTlsMode(body.tls_mode || 'starttls');
        setHasPassword(body.has_password);
        setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/settings/smtp', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          host,
          port: Number(port),
          username,
          password,
          from_address: fromAddress,
          tls_mode: tlsMode,
        }),
      });
      if (!res.ok) throw new Error('save failed');
      if (password) {
        setHasPassword(true);
        setPassword('');
      }
      setStatus('saved');
    } catch {
      setStatus('error');
    }
  }

  async function handleSendTest() {
    setTestStatus('saving');
    setTestError(null);
    try {
      const res = await apiFetch('/api/admin/settings/smtp/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ to: testTo }),
      });
      if (!res.ok) {
        setTestError((await res.text()) || 'Failed to send test email.');
        setTestStatus('error');
        return;
      }
      setTestStatus('saved');
    } catch {
      setTestError('Could not reach the server.');
      setTestStatus('error');
    }
  }

  if (!loaded) {
    return null;
  }

  return (
    <div className="settings-card">
      <h2>Email (SMTP)</h2>
      <p className="settings-form-help">Used to send password reset links to admins.</p>
      <form className="settings-form" onSubmit={handleSubmit}>
        <div className="settings-form-row">
          <label className="field">
            <span className="kicker">Host</span>
            <input value={host} onChange={(e) => setHost(e.target.value)} required />
          </label>
          <label className="field">
            <span className="kicker">Port</span>
            <input
              type="number"
              min={1}
              max={65535}
              value={port}
              onChange={(e) => setPort(e.target.value)}
              required
            />
          </label>
        </div>
        <label className="field">
          <span className="kicker">Username</span>
          <input value={username} onChange={(e) => setUsername(e.target.value)} />
        </label>
        <label className="field">
          <span className="kicker">Password{hasPassword ? ' (set)' : ''}</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={hasPassword ? 'Leave blank to keep current password' : ''}
          />
        </label>
        <label className="field">
          <span className="kicker">From address</span>
          <input
            type="email"
            value={fromAddress}
            onChange={(e) => setFromAddress(e.target.value)}
            required
          />
        </label>
        <label className="field">
          <span className="kicker">Encryption</span>
          <select value={tlsMode} onChange={(e) => setTlsMode(e.target.value)}>
            <option value="none">None</option>
            <option value="starttls">STARTTLS</option>
            <option value="tls">TLS</option>
          </select>
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
        <label className="field">
          <span className="kicker">Send test email to (optional)</span>
          <input
            type="email"
            value={testTo}
            onChange={(e) => {
              setTestTo(e.target.value);
              setTestStatus('idle');
            }}
            placeholder="Defaults to your account email"
          />
        </label>
        <div className="settings-form-actions">
          <button
            type="button"
            className="btn-primary"
            onClick={handleSendTest}
            disabled={testStatus === 'saving'}
          >
            {testStatus === 'saving' ? 'Sending…' : 'Send test email'}
          </button>
          {testStatus === 'saved' && <span className="save-note ok">Sent.</span>}
          {testStatus === 'error' && (
            <span className="save-note error" role="alert">
              {testError}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

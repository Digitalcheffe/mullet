import { useEffect, useState, type FormEvent } from 'react';
import { useSearchParams } from 'react-router-dom';
import { QRCodeSVG } from 'qrcode.react';
import { useApiFetch } from '../auth/useApiFetch';
import './SettingsPage.css';

const SETTINGS_TABS = [
  { id: 'general', label: 'General' },
  { id: 'account', label: 'Account' },
  { id: 'users', label: 'Users' },
  { id: 'notifications', label: 'Notifications' },
] as const;
type SettingsTab = (typeof SETTINGS_TABS)[number]['id'];

interface Settings {
  server_name: string;
  port: string;
  db_path: string;
  log_file_path: string;
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
  const [logFilePath, setLogFilePath] = useState('');
  const [status, setStatus] = useState<SaveStatus>('idle');
  const [error, setError] = useState<string | null>(null);

  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab');
  const activeTab: SettingsTab = SETTINGS_TABS.some((t) => t.id === requestedTab)
    ? (requestedTab as SettingsTab)
    : 'general';

  function selectTab(tab: SettingsTab) {
    setSearchParams(tab === 'general' ? {} : { tab });
  }

  useEffect(() => {
    let cancelled = false;

    apiFetch('/api/admin/settings')
      .then((res) => res.json())
      .then((body: Settings) => {
        if (cancelled) return;
        setSettings(body);
        setServerName(body.server_name);
        setLogFilePath(body.log_file_path);
      });

    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_name: serverName, log_file_path: logFilePath }),
      });
      if (!res.ok) {
        setError((await res.text()) || 'Failed to save.');
        setStatus('error');
        return;
      }
      setStatus('saved');
    } catch {
      setError('Could not reach the server.');
      setStatus('error');
    }
  }

  if (!settings) {
    return <p>Loading…</p>;
  }

  return (
    <div className="settings-page">
      <h1>Settings</h1>

      <div className="settings-tabs" role="tablist">
        {SETTINGS_TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            className={activeTab === tab.id ? 'settings-tab active' : 'settings-tab'}
            onClick={() => selectTab(tab.id)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'general' && (
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
            <label className="field">
              <span className="kicker">Log file path</span>
              <input
                value={logFilePath}
                onChange={(e) => {
                  setLogFilePath(e.target.value);
                  setStatus('idle');
                }}
                placeholder="Leave blank to log to stdout only"
              />
              <span className="field-help">
                When set, logs are written to both stdout and this file. The path must be writable
                by the server process.
              </span>
            </label>
            <div className="settings-form-actions">
              <button type="submit" className="btn-primary" disabled={status === 'saving'}>
                {status === 'saving' ? 'Saving…' : 'Save'}
              </button>
              {status === 'saved' && <span className="save-note ok">Saved.</span>}
              {status === 'error' && (
                <span className="save-note error" role="alert">
                  {error}
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
      )}

      {activeTab === 'account' && (
        <>
          <AccountSection apiFetch={apiFetch} />
          <TwoFactorSection apiFetch={apiFetch} />
        </>
      )}

      {activeTab === 'users' && <UsersSection apiFetch={apiFetch} />}

      {activeTab === 'notifications' && (
        <>
          <SMTPSection apiFetch={apiFetch} />
          <WebhooksSection apiFetch={apiFetch} />
          <NotificationsSection apiFetch={apiFetch} />
        </>
      )}
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

      <PasswordChangeForm apiFetch={apiFetch} />
    </div>
  );
}

function PasswordChangeForm({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [status, setStatus] = useState<SaveStatus>('idle');
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (newPassword !== confirmPassword) {
      setError('New passwords do not match.');
      setStatus('error');
      return;
    }
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/account/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      if (!res.ok) {
        setError((await res.text()) || 'Failed to change password.');
        setStatus('error');
        return;
      }
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setStatus('saved');
    } catch {
      setError('Could not reach the server.');
      setStatus('error');
    }
  }

  return (
    <form className="settings-form" onSubmit={handleSubmit}>
      <span className="kicker">Change password</span>
      <label className="field">
        <span className="kicker">Current password</span>
        <input
          type="password"
          value={currentPassword}
          onChange={(e) => {
            setCurrentPassword(e.target.value);
            setStatus('idle');
          }}
          required
        />
      </label>
      <label className="field">
        <span className="kicker">New password</span>
        <input
          type="password"
          value={newPassword}
          onChange={(e) => {
            setNewPassword(e.target.value);
            setStatus('idle');
          }}
          required
          minLength={8}
        />
      </label>
      <label className="field">
        <span className="kicker">Confirm new password</span>
        <input
          type="password"
          value={confirmPassword}
          onChange={(e) => {
            setConfirmPassword(e.target.value);
            setStatus('idle');
          }}
          required
          minLength={8}
        />
      </label>
      <div className="settings-form-actions">
        <button type="submit" className="btn-primary" disabled={status === 'saving'}>
          {status === 'saving' ? 'Changing…' : 'Change password'}
        </button>
        {status === 'saved' && <span className="save-note ok">Password changed.</span>}
        {status === 'error' && (
          <span className="save-note error" role="alert">
            {error}
          </span>
        )}
      </div>
    </form>
  );
}

interface TOTPStatus {
  enabled: boolean;
  backup_codes_remaining: number;
}

type TwoFactorStep = 'status' | 'enroll' | 'disable' | 'regenerate';

function TwoFactorSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [totpStatus, setTOTPStatus] = useState<TOTPStatus | null>(null);
  const [step, setStep] = useState<TwoFactorStep>('status');
  const [backupCodes, setBackupCodes] = useState<string[] | null>(null);

  function reloadStatus() {
    return apiFetch('/api/admin/account/totp')
      .then((res) => res.json())
      .then((body: TOTPStatus) => setTOTPStatus(body));
  }

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/account/totp')
      .then((res) => res.json())
      .then((body: TOTPStatus) => {
        if (!cancelled) setTOTPStatus(body);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  // Enrollment step's own state.
  const [secret, setSecret] = useState('');
  const [authURL, setAuthURL] = useState('');
  const [enrollLoaded, setEnrollLoaded] = useState(false);
  const [code, setCode] = useState('');
  const [enrollError, setEnrollError] = useState<string | null>(null);
  const [enrollSubmitting, setEnrollSubmitting] = useState(false);

  async function startEnroll() {
    setStep('enroll');
    setEnrollLoaded(false);
    setEnrollError(null);
    setCode('');
    const res = await apiFetch('/api/admin/account/totp/enroll', { method: 'POST' });
    if (!res.ok) {
      setEnrollError((await res.text()) || 'Failed to start enrollment.');
      setEnrollLoaded(true);
      return;
    }
    const body: { secret: string; auth_url: string } = await res.json();
    setSecret(body.secret);
    setAuthURL(body.auth_url);
    setEnrollLoaded(true);
  }

  async function confirmEnroll(e: FormEvent) {
    e.preventDefault();
    setEnrollError(null);
    setEnrollSubmitting(true);
    try {
      const res = await apiFetch('/api/admin/account/totp/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code }),
      });
      if (!res.ok) {
        setEnrollError((await res.text()) || 'That code didn’t match.');
        return;
      }
      const body: { backup_codes: string[] } = await res.json();
      setBackupCodes(body.backup_codes);
      await reloadStatus();
      setStep('status');
    } catch {
      setEnrollError('Could not reach the server.');
    } finally {
      setEnrollSubmitting(false);
    }
  }

  // Disable / regenerate steps share a "confirm with current password"
  // form -- both are security-lowering actions.
  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [passwordSubmitting, setPasswordSubmitting] = useState(false);

  async function submitPasswordConfirm(e: FormEvent) {
    e.preventDefault();
    setPasswordError(null);
    setPasswordSubmitting(true);
    try {
      if (step === 'disable') {
        const res = await apiFetch('/api/admin/account/totp', {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password }),
        });
        if (!res.ok) {
          setPasswordError((await res.text()) || 'Failed to disable.');
          return;
        }
        await reloadStatus();
      } else if (step === 'regenerate') {
        const res = await apiFetch('/api/admin/account/totp/backup-codes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password }),
        });
        if (!res.ok) {
          setPasswordError((await res.text()) || 'Failed to regenerate.');
          return;
        }
        const body: { backup_codes: string[] } = await res.json();
        setBackupCodes(body.backup_codes);
      }
      setPassword('');
      setStep('status');
    } catch {
      setPasswordError('Could not reach the server.');
    } finally {
      setPasswordSubmitting(false);
    }
  }

  function cancelStep() {
    setStep('status');
    setPassword('');
    setPasswordError(null);
    setEnrollError(null);
  }

  if (!totpStatus) {
    return null;
  }

  return (
    <div className="settings-card">
      <h2>Two-factor authentication</h2>

      {backupCodes && (
        <div className="totp-backup-codes">
          <p className="settings-form-help">
            Save these backup codes somewhere safe -- each works once, and this is the only time
            they'll be shown. Use one to sign in if you lose access to your authenticator app.
          </p>
          <ul className="backup-codes-list">
            {backupCodes.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
          <div className="settings-form-actions">
            <button type="button" className="btn-primary" onClick={() => setBackupCodes(null)}>
              I've saved these codes
            </button>
          </div>
        </div>
      )}

      {step === 'status' && !backupCodes && (
        <>
          <p className="settings-form-help">
            {totpStatus.enabled
              ? `Enabled. ${totpStatus.backup_codes_remaining} backup code${totpStatus.backup_codes_remaining === 1 ? '' : 's'} remaining.`
              : "Not enabled. Adds a 6-digit code from an authenticator app to sign-in, on top of your password."}
          </p>
          <div className="settings-form-actions">
            {!totpStatus.enabled && (
              <button type="button" className="btn-primary" onClick={startEnroll}>
                Enable
              </button>
            )}
            {totpStatus.enabled && (
              <>
                <button type="button" className="btn-secondary" onClick={() => setStep('regenerate')}>
                  Regenerate backup codes
                </button>
                <button type="button" className="btn-danger" onClick={() => setStep('disable')}>
                  Disable
                </button>
              </>
            )}
          </div>
        </>
      )}

      {step === 'enroll' && (
        <form className="settings-form" onSubmit={confirmEnroll}>
          {!enrollLoaded && <p>Loading…</p>}
          {enrollLoaded && enrollError && !secret && (
            <p className="form-error" role="alert">
              {enrollError}
            </p>
          )}
          {enrollLoaded && secret && (
            <>
              <p className="settings-form-help">
                Scan this with your authenticator app (Google Authenticator, Authy, 1Password, ...),
                or enter the key manually.
              </p>
              <QRCodeSVG value={authURL} size={180} />
              <label className="field">
                <span className="kicker">Manual entry key</span>
                <div className="readonly-value">{secret}</div>
              </label>
              <label className="field">
                <span className="kicker">Enter the 6-digit code to confirm</span>
                <input
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  required
                  autoFocus
                  autoComplete="one-time-code"
                />
              </label>
              {enrollError && (
                <p className="form-error" role="alert">
                  {enrollError}
                </p>
              )}
              <div className="settings-form-actions">
                <button type="submit" className="btn-primary" disabled={enrollSubmitting}>
                  {enrollSubmitting ? 'Confirming…' : 'Confirm'}
                </button>
                <button type="button" className="btn-secondary" onClick={cancelStep}>
                  Cancel
                </button>
              </div>
            </>
          )}
        </form>
      )}

      {(step === 'disable' || step === 'regenerate') && (
        <form className="settings-form" onSubmit={submitPasswordConfirm}>
          <p className="settings-form-help">
            {step === 'disable'
              ? 'Enter your current password to disable two-factor authentication.'
              : 'Enter your current password to regenerate backup codes -- the old ones stop working immediately.'}
          </p>
          <label className="field">
            <span className="kicker">Current password</span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoFocus
            />
          </label>
          {passwordError && (
            <p className="form-error" role="alert">
              {passwordError}
            </p>
          )}
          <div className="settings-form-actions">
            <button type="submit" className="btn-danger" disabled={passwordSubmitting}>
              {passwordSubmitting ? 'Working…' : step === 'disable' ? 'Disable' : 'Regenerate'}
            </button>
            <button type="button" className="btn-secondary" onClick={cancelStep}>
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  );
}

interface AdminUser {
  id: number;
  username: string;
  email: string | null;
  created_at: string;
}

function UsersSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [status, setStatus] = useState<SaveStatus>('idle');
  const [error, setError] = useState<string | null>(null);

  function loadUsers() {
    return apiFetch('/api/admin/users')
      .then((res) => res.json())
      .then((body: AdminUser[]) => setUsers(body));
  }

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/users')
      .then((res) => res.json())
      .then((body: AdminUser[]) => {
        if (!cancelled) setUsers(body);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) {
        setError((await res.text()) || 'Failed to add account.');
        setStatus('error');
        return;
      }
      setUsername('');
      setPassword('');
      setStatus('saved');
      await loadUsers();
    } catch {
      setError('Could not reach the server.');
      setStatus('error');
    }
  }

  async function handleRemove(id: number) {
    setError(null);
    try {
      const res = await apiFetch(`/api/admin/users/${id}`, { method: 'DELETE' });
      if (!res.ok) {
        setError((await res.text()) || 'Failed to remove account.');
        return;
      }
      await loadUsers();
    } catch {
      setError('Could not reach the server.');
    }
  }

  if (!users) {
    return null;
  }

  return (
    <div className="settings-card">
      <h2>Users</h2>
      <p className="settings-form-help">
        Every admin account has equal access -- there are no permission levels.
      </p>
      {error && (
        <span className="save-note error" role="alert">
          {error}
        </span>
      )}

      <ul className="users-list">
        {users.map((u) => (
          <li key={u.id} className="users-list-row">
            <div>
              <div className="users-list-username">{u.username}</div>
              <div className="users-list-meta">Added {new Date(u.created_at).toLocaleDateString()}</div>
            </div>
            <button
              type="button"
              className="btn-danger"
              onClick={() => handleRemove(u.id)}
              disabled={users.length <= 1}
              title={users.length <= 1 ? "Can't remove the last remaining admin account" : undefined}
            >
              Remove
            </button>
          </li>
        ))}
      </ul>

      <form className="settings-form" onSubmit={handleAdd}>
        <span className="kicker">Add an admin</span>
        <label className="field">
          <span className="kicker">Username</span>
          <input
            value={username}
            onChange={(e) => {
              setUsername(e.target.value);
              setStatus('idle');
            }}
            required
          />
        </label>
        <label className="field">
          <span className="kicker">Password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value);
              setStatus('idle');
            }}
            required
            minLength={8}
          />
        </label>
        <div className="settings-form-actions">
          <button type="submit" className="btn-primary" disabled={status === 'saving'}>
            {status === 'saving' ? 'Adding…' : 'Add admin'}
          </button>
          {status === 'saved' && <span className="save-note ok">Added.</span>}
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

interface WebhookConfig {
  url: string;
  has_secret: boolean;
  payload_template: string;
}

function WebhooksSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [loaded, setLoaded] = useState(false);
  const [hasSecret, setHasSecret] = useState(false);
  const [url, setUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [payloadTemplate, setPayloadTemplate] = useState('');
  const [status, setStatus] = useState<SaveStatus>('idle');
  const [error, setError] = useState<string | null>(null);

  const [testStatus, setTestStatus] = useState<SaveStatus>('idle');
  const [testError, setTestError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/settings/webhook')
      .then((res) => res.json())
      .then((body: WebhookConfig) => {
        if (cancelled) return;
        setUrl(body.url);
        setHasSecret(body.has_secret);
        setPayloadTemplate(body.payload_template);
        setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/settings/webhook', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url, secret, payload_template: payloadTemplate }),
      });
      if (!res.ok) {
        setError((await res.text()) || 'Failed to save.');
        setStatus('error');
        return;
      }
      if (secret) {
        setHasSecret(true);
        setSecret('');
      }
      setStatus('saved');
    } catch {
      setError('Could not reach the server.');
      setStatus('error');
    }
  }

  async function handleSendTest() {
    setTestStatus('saving');
    setTestError(null);
    try {
      const res = await apiFetch('/api/admin/settings/webhook/test', { method: 'POST' });
      if (!res.ok) {
        setTestError((await res.text()) || 'Failed to deliver.');
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
      <h2>Webhooks</h2>
      <p className="settings-form-help">Deliver notification events to an HTTP endpoint as JSON.</p>
      <form className="settings-form" onSubmit={handleSubmit}>
        <label className="field">
          <span className="kicker">URL</span>
          <input
            type="url"
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              setStatus('idle');
            }}
            placeholder="https://example.com/webhook"
          />
        </label>
        <label className="field">
          <span className="kicker">Signing secret{hasSecret ? ' (set)' : ''}</span>
          <input
            type="password"
            value={secret}
            onChange={(e) => {
              setSecret(e.target.value);
              setStatus('idle');
            }}
            placeholder={hasSecret ? 'Leave blank to keep current secret' : 'Optional'}
          />
          <span className="field-help">
            If set, each request is signed with an X-Mullet-Signature: sha256=... header (HMAC-SHA256
            over the raw body).
          </span>
        </label>
        <label className="field">
          <span className="kicker">Payload template</span>
          <textarea
            className="webhook-template"
            value={payloadTemplate}
            onChange={(e) => {
              setPayloadTemplate(e.target.value);
              setStatus('idle');
            }}
            rows={6}
            spellCheck={false}
          />
          <span className="field-help">
            Must render to valid JSON. Variables: {'{event}'}, {'{subject}'}, {'{message}'}, {'{timestamp}'}
            -- each is JSON-escaped automatically, so leave them inside quotes.
          </span>
        </label>
        <div className="settings-form-actions">
          <button type="submit" className="btn-primary" disabled={status === 'saving'}>
            {status === 'saving' ? 'Saving…' : 'Save'}
          </button>
          {status === 'saved' && <span className="save-note ok">Saved.</span>}
          {status === 'error' && (
            <span className="save-note error" role="alert">
              {error}
            </span>
          )}
        </div>
      </form>

      <div className="settings-readonly">
        <div className="settings-form-actions">
          <button type="button" className="btn-primary" onClick={handleSendTest} disabled={testStatus === 'saving'}>
            {testStatus === 'saving' ? 'Sending…' : 'Send test webhook'}
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

interface NotificationPreferences {
  new_user: boolean;
  password_reset_requested: boolean;
  password_reset_completed: boolean;
  client_registered: boolean;
  client_approved: boolean;
}

const NOTIFICATION_EVENTS: { key: keyof NotificationPreferences; label: string }[] = [
  { key: 'new_user', label: 'A new admin account is created' },
  { key: 'password_reset_requested', label: 'A password reset is requested' },
  { key: 'password_reset_completed', label: 'A password reset is completed' },
  { key: 'client_registered', label: 'A new display client registers' },
  { key: 'client_approved', label: 'A pending client is approved' },
];

function NotificationsSection({ apiFetch }: { apiFetch: ReturnType<typeof useApiFetch> }) {
  const [prefs, setPrefs] = useState<NotificationPreferences | null>(null);
  const [status, setStatus] = useState<SaveStatus>('idle');

  useEffect(() => {
    let cancelled = false;
    apiFetch('/api/admin/settings/notifications')
      .then((res) => res.json())
      .then((body: NotificationPreferences) => {
        if (!cancelled) setPrefs(body);
      });
    return () => {
      cancelled = true;
    };
  }, [apiFetch]);

  async function handleToggle(key: keyof NotificationPreferences, value: boolean) {
    if (!prefs) return;
    const next = { ...prefs, [key]: value };
    setPrefs(next);
    setStatus('saving');
    try {
      const res = await apiFetch('/api/admin/settings/notifications', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(next),
      });
      if (!res.ok) throw new Error('save failed');
      setStatus('saved');
    } catch {
      setStatus('error');
    }
  }

  if (!prefs) {
    return null;
  }

  return (
    <div className="settings-card">
      <h2>Notifications</h2>
      <p className="settings-form-help">
        Notify every admin with an address on file, and/or deliver to a webhook, when one of
        these happens. Requires SMTP and/or a webhook URL to be configured above.
      </p>
      <div className="settings-form">
        {NOTIFICATION_EVENTS.map(({ key, label }) => (
          <label className="toggle-row" key={key}>
            <span>{label}</span>
            <input
              type="checkbox"
              checked={prefs[key]}
              onChange={(e) => handleToggle(key, e.target.checked)}
            />
          </label>
        ))}
        <div className="settings-form-actions">
          {status === 'saved' && <span className="save-note ok">Saved.</span>}
          {status === 'error' && (
            <span className="save-note error" role="alert">
              Failed to save.
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

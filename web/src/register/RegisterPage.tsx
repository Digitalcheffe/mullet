import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { deleteCookie, getCookie, setCookie } from '../shared/cookies';
import './RegisterPage.css';

// RegisterPage is the raw-browser stand-in for a dedicated client app
// (see architecture.md's Client Connection Model): it drives the same
// POST /api/clients/register -> admin approves -> GET .../config
// flow, just from inside the browser itself instead of a separate
// installed app. The client_id it generates is a pairing code exactly
// like a real client would show -- persisted in a cookie (not an auth
// cookie, just "remember which pairing this browser already did")
// rather than localStorage so a kiosk browser configured to clear site
// data on exit can still be told to keep it, the same knob it'd use
// for any other cookie it wants to survive.
const CLIENT_ID_COOKIE = 'mullet_client_id';
const CLIENT_NAME_COOKIE = 'mullet_client_name';
const COOKIE_DAYS = 400; // ~13 months -- the practical max most browsers allow anyway
const POLL_MS = 5000;

// Pairing code alphabet skips visually ambiguous characters (0/O,
// 1/I/L) since this is meant to be read off a screen and typed nowhere
// -- the admin only ever needs to recognize it, not transcribe it, but
// there's no reason to make that harder than it has to be.
const PAIRING_CHARSET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

function generateClientId(): string {
  const bytes = new Uint8Array(6);
  crypto.getRandomValues(bytes);
  const chars = Array.from(bytes, (b) => PAIRING_CHARSET[b % PAIRING_CHARSET.length]).join('');
  return `${chars.slice(0, 3)}-${chars.slice(3, 6)}`;
}

interface ClientConfig {
  status: 'pending' | 'approved' | 'rejected';
  display_slug?: string;
}

export default function RegisterPage() {
  const navigate = useNavigate();
  const [clientId, setClientId] = useState<string | null>(() => getCookie(CLIENT_ID_COOKIE));
  const [name, setName] = useState(() => getCookie(CLIENT_NAME_COOKIE) ?? '');
  const [registering, setRegistering] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<ClientConfig['status'] | null>(null);

  const register = useCallback(async (id: string, displayName: string) => {
    setError(null);
    try {
      const res = await fetch('/api/clients/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_id: id, name: displayName, platform: 'browser' }),
      });
      if (!res.ok) throw new Error((await res.text()) || 'Registration failed');
      setCookie(CLIENT_ID_COOKIE, id, COOKIE_DAYS);
      setCookie(CLIENT_NAME_COOKIE, displayName, COOKIE_DAYS);
      setClientId(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    }
  }, []);

  // A returning visit with both cookies already set re-sends the same
  // registration (idempotent server-side, see docs on POST
  // .../register) rather than skipping straight to polling -- covers
  // the server having lost its record of this client_id (a fresh DB, a
  // restored backup) without the browser needing to notice and recover
  // on its own.
  useEffect(() => {
    if (clientId && name) register(clientId, name);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!clientId) return;
    let cancelled = false;

    async function poll() {
      try {
        const res = await fetch(`/api/clients/${encodeURIComponent(clientId!)}/config`);
        if (res.status === 404) {
          // No server-side record of this client_id -- a stale cookie
          // from before a DB reset, or the admin deleted it outright.
          // Drop back to the name form rather than polling forever.
          if (!cancelled) {
            deleteCookie(CLIENT_ID_COOKIE);
            deleteCookie(CLIENT_NAME_COOKIE);
            setClientId(null);
            setStatus(null);
          }
          return;
        }
        if (!res.ok) return;
        const cfg: ClientConfig = await res.json();
        if (cancelled) return;
        setStatus(cfg.status);
        if (cfg.status === 'approved' && cfg.display_slug) {
          navigate(`/display/${cfg.display_slug}`, { replace: true });
        }
      } catch {
        // Transient network error -- the next tick just tries again,
        // same as a real client's own polling backoff.
      }
    }

    poll();
    const interval = setInterval(poll, POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [clientId, navigate]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setRegistering(true);
    register(generateClientId(), name.trim()).finally(() => setRegistering(false));
  }

  function handleForget() {
    deleteCookie(CLIENT_ID_COOKIE);
    deleteCookie(CLIENT_NAME_COOKIE);
    setClientId(null);
    setStatus(null);
    setName('');
  }

  if (!clientId) {
    return (
      <div className="register-page">
        <form className="register-card" onSubmit={handleSubmit}>
          <h1>Register This Browser</h1>
          <p>Give this display a name. An admin approves it and assigns it a dashboard from the Clients page.</p>
          {error && (
            <p className="register-error" role="alert">
              {error}
            </p>
          )}
          <input
            className="register-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Kitchen"
            autoFocus
            required
          />
          <button className="register-button" type="submit" disabled={registering}>
            {registering ? 'Registering…' : 'Register'}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div className="register-page">
      <div className="register-card">
        <span className={`register-status${status === 'rejected' ? ' rejected' : ''}`}>
          {status === 'rejected' ? 'Rejected' : 'Waiting for Approval'}
        </span>
        <div className="register-code">{clientId}</div>
        <p className="register-name">{name}</p>
        {status === 'rejected' ? (
          <p className="register-hint">This registration was rejected. An admin can still approve it later from the Clients page.</p>
        ) : (
          <p className="register-hint">Approve this client on the admin's Clients page, using the code above.</p>
        )}
        <button className="register-forget" onClick={handleForget}>
          Not this device? Register again
        </button>
      </div>
    </div>
  );
}

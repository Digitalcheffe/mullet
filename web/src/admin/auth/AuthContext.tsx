import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';

// Returned by login() so LoginPage can tell a completed login apart
// from one that still needs a TOTP/backup code (issue #114) --
// pendingToken is only set in the second case.
export interface LoginResult {
  mfaRequired: boolean;
  pendingToken?: string;
}

// Persisted in sessionStorage (issue #168) -- cleared when the tab
// closes, unlike localStorage, so it doesn't linger as a stored
// credential past the browser session, but survives the reload a back/
// forward navigation or manual refresh causes within it. The token
// itself already carries a real 24h server-side expiry (see tokenTTL
// in internal/auth/jwt.go); a stale-but-still-stored one needs no
// special handling here since useApiFetch already logs out on any 401.
const SESSION_STORAGE_KEY = 'mullet-admin-session';

interface StoredSession {
  token: string;
  username: string;
}

function readStoredSession(): StoredSession | null {
  try {
    const raw = sessionStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (typeof parsed?.token === 'string' && typeof parsed?.username === 'string') {
      return parsed;
    }
    return null;
  } catch {
    return null;
  }
}

function writeStoredSession(session: StoredSession | null): void {
  try {
    if (session) {
      sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
    } else {
      sessionStorage.removeItem(SESSION_STORAGE_KEY);
    }
  } catch {
    // Storage unavailable (private browsing, quota, etc.) -- session
    // simply won't survive a reload, same as before this fix.
  }
}

interface AuthContextValue {
  token: string | null;
  username: string | null;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<LoginResult>;
  // Exchanges a pending token + a TOTP or backup code for a real
  // session, once login() has reported mfaRequired.
  verifyMFA: (username: string, pendingToken: string, code: string) => Promise<void>;
  logout: () => void;
  // Sets an already-issued token (e.g. returned by the setup wizard)
  // without going through the login endpoint again.
  setSession: (token: string, username: string) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => readStoredSession()?.token ?? null);
  const [username, setUsername] = useState<string | null>(() => readStoredSession()?.username ?? null);

  const setSession = useCallback((newToken: string, newUsername: string) => {
    setToken(newToken);
    setUsername(newUsername);
    writeStoredSession({ token: newToken, username: newUsername });
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setUsername(null);
    writeStoredSession(null);
  }, []);

  const login = useCallback(
    async (loginUsername: string, password: string): Promise<LoginResult> => {
      const res = await fetch('/api/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername, password }),
      });
      if (!res.ok) {
        throw new Error('Invalid username or password');
      }
      const body: { token?: string; mfa_required?: boolean; pending_token?: string } = await res.json();
      if (body.mfa_required) {
        return { mfaRequired: true, pendingToken: body.pending_token };
      }
      setSession(body.token!, loginUsername);
      return { mfaRequired: false };
    },
    [setSession],
  );

  const verifyMFA = useCallback(
    async (loginUsername: string, pendingToken: string, code: string) => {
      const res = await fetch('/api/admin/mfa/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pending_token: pendingToken, code }),
      });
      if (!res.ok) {
        throw new Error('Invalid code');
      }
      const body: { token: string } = await res.json();
      setSession(body.token, loginUsername);
    },
    [setSession],
  );

  const value = useMemo(
    () => ({ token, username, isAuthenticated: token !== null, login, verifyMFA, logout, setSession }),
    [token, username, login, verifyMFA, logout, setSession],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}

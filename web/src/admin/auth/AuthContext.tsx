import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';

interface AuthContextValue {
  token: string | null;
  username: string | null;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  // Sets an already-issued token (e.g. returned by the setup wizard)
  // without going through the login endpoint again.
  setSession: (token: string, username: string) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [username, setUsername] = useState<string | null>(null);

  const setSession = useCallback((newToken: string, newUsername: string) => {
    setToken(newToken);
    setUsername(newUsername);
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setUsername(null);
  }, []);

  const login = useCallback(
    async (loginUsername: string, password: string) => {
      const res = await fetch('/api/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername, password }),
      });
      if (!res.ok) {
        throw new Error('Invalid username or password');
      }
      const body: { token: string } = await res.json();
      setSession(body.token, loginUsername);
    },
    [setSession],
  );

  const value = useMemo(
    () => ({ token, username, isAuthenticated: token !== null, login, logout, setSession }),
    [token, username, login, logout, setSession],
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

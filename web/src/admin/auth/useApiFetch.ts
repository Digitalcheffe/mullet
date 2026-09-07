import { useCallback } from 'react';
import { useAuth } from './AuthContext';

// Wraps fetch with the current admin session's bearer token attached, and
// logs the session out (falling back to the login page) on a 401 response.
export function useApiFetch() {
  const { token, logout } = useAuth();

  return useCallback(
    async (path: string, init: RequestInit = {}) => {
      const res = await fetch(path, {
        ...init,
        headers: {
          ...init.headers,
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });

      if (res.status === 401) {
        logout();
      }

      return res;
    },
    [token, logout],
  );
}

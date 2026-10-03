import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api } from '../api/api';
import { onUnauthorized } from '../api/client';
import type { Me } from '../api/types';
import { AuthContext, type AuthState } from './authContext';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => {
        setUser(null);
      })
      .finally(() => {
        setLoading(false);
      });
    return onUnauthorized(() => {
      setUser(null);
    });
  }, []);

  const login = useCallback(async (userId: string, password: string) => {
    await api.login(userId, password);
    setUser(await api.me());
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setUser(null);
    }
  }, []);

  const value = useMemo<AuthState>(
    () => ({ user, loading, login, logout, can: (p) => user?.permissions.includes(p) ?? false }),
    [user, loading, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

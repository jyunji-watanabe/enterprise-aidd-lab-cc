import { createContext, useContext } from 'react';
import type { Me, Permission } from '../api/types';

export interface AuthState {
  user: Me | null;
  loading: boolean;
  login: (userId: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  can: (p: Permission) => boolean;
}

export const AuthContext = createContext<AuthState | null>(null);

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}

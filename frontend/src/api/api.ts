import { del, get, post, put } from './client';
import type {
  Account,
  Action,
  Attachment,
  AuditLog,
  Department,
  Expense,
  ExpenseDetail,
  ExpenseInput,
  Me,
  ReminderLog,
  ReminderResult,
  Role,
  Status,
  User,
} from './types';

export type ExpenseScope = 'mine' | 'approvals' | 'all';

export interface AccountPayload {
  code: string;
  name: string;
  active: boolean;
  limitAmount: number | null;
}

export interface UserPayload {
  id?: string;
  name: string;
  departmentId: number;
  role: Role;
  password: string;
}

const qs = (params: Record<string, string | number | undefined>) => {
  const s = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') s.set(k, String(v));
  });
  const str = s.toString();
  return str ? `?${str}` : '';
};

export const api = {
  login: (userId: string, password: string) => post<User>('/api/auth/login', { userId, password }),
  logout: () => post<undefined>('/api/auth/logout'),
  me: () => get<Me>('/api/auth/me'),

  departments: () => get<Department[]>('/api/departments'),
  createDepartment: (d: Omit<Department, 'id'>) => post<Department>('/api/departments', d),
  updateDepartment: (id: number, d: Omit<Department, 'id'>) => put<Department>(`/api/departments/${id}`, d),
  deleteDepartment: (id: number) => del(`/api/departments/${id}`),

  accounts: (all = false) => get<Account[]>(`/api/accounts${all ? '?all=1' : ''}`),
  createAccount: (a: AccountPayload) => post<Account>('/api/accounts', a),
  updateAccount: (id: number, a: AccountPayload) => put<Account>(`/api/accounts/${id}`, a),
  deleteAccount: (id: number) => del(`/api/accounts/${id}`),

  users: () => get<User[]>('/api/users'),
  createUser: (u: UserPayload) => post<User>('/api/users', u),
  updateUser: (id: string, u: UserPayload) => put<User>(`/api/users/${encodeURIComponent(id)}`, u),
  deleteUser: (id: string) => del(`/api/users/${encodeURIComponent(id)}`),

  expenses: (scope: ExpenseScope, status?: Status | '') =>
    get<Expense[]>(`/api/expenses${qs({ scope, status })}`),
  expense: (id: number) => get<ExpenseDetail>(`/api/expenses/${id}`),
  createExpense: (input: ExpenseInput) => post<ExpenseDetail>('/api/expenses', input),
  updateExpense: (id: number, input: ExpenseInput) => put<ExpenseDetail>(`/api/expenses/${id}`, input),
  transition: (id: number, action: Action, comment: string) =>
    post<ExpenseDetail>(`/api/expenses/${id}/${action}`, { comment }),

  uploadAttachment: (file: File) => {
    const fd = new FormData();
    fd.append('file', file);
    return post<Attachment>('/api/attachments', fd);
  },
  attachmentUrl: (id: number) => `/api/attachments/${id}`,

  auditLogs: (params: { action?: string; actorId?: string; limit?: number; offset?: number }) =>
    get<{ items: AuditLog[]; total: number }>(`/api/admin/audit-logs${qs(params)}`),
  verifyAudit: () =>
    get<{ valid: boolean; checked: number; brokenAt?: number }>('/api/admin/audit-logs/verify'),
  settledCsvUrl: (from: string, to: string) => `/api/admin/exports/settled.csv${qs({ from, to })}`,
  runReminders: () => post<ReminderResult>('/api/admin/batch/reminders'),
  reminders: () => get<ReminderLog[]>('/api/admin/batch/reminders'),
};

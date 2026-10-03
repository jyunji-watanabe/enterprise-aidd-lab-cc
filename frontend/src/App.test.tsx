import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Me } from './api/types';
import { AppRoutes } from './App';
import { AuthProvider } from './auth/AuthContext';

const employee: Me = {
  id: 'employee1',
  name: '山田 太郎',
  departmentId: 1,
  departmentName: '営業部',
  role: 'Employee',
  createdAt: '',
  updatedAt: '',
  permissions: [],
};

function stubApi(me: Me | null) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/auth/me') {
        return Promise.resolve(
          me
            ? new Response(JSON.stringify(me), { status: 200 })
            : new Response(JSON.stringify({ error: '認証が必要です' }), { status: 401 }),
        );
      }
      return Promise.resolve(new Response('[]', { status: 200 }));
    }),
  );
}

function renderAt(path: string) {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe('routing and permissions', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('redirects anonymous users to the login page', async () => {
    stubApi(null);
    renderAt('/expenses');
    expect(await screen.findByRole('button', { name: 'ログイン' })).toBeInTheDocument();
  });

  it('hides admin navigation from employees and blocks admin screens', async () => {
    stubApi(employee);
    renderAt('/admin/masters');
    expect(await screen.findByText('この画面を表示する権限がありません。')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'マスタ管理' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: '監査ログ' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: '自分の申請' })).toBeInTheDocument();
  });

  it('shows admin navigation to admins', async () => {
    stubApi({
      ...employee,
      role: 'Admin',
      permissions: ['manage_masters', 'view_audit_log', 'export_reports'],
    });
    renderAt('/expenses');
    expect(await screen.findByRole('link', { name: 'マスタ管理' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '監査ログ' })).toBeInTheDocument();
  });
});

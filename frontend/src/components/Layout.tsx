import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import { roleLabel } from '../domain/labels';

export function Layout() {
  const { user, can, logout } = useAuth();
  const navigate = useNavigate();
  if (!user) return null;

  const onLogout = async () => {
    await logout();
    void navigate('/login');
  };

  return (
    <div className="app">
      <header className="app-header no-print">
        <div className="brand">経費精算システム</div>
        <nav>
          <NavLink to="/expenses">自分の申請</NavLink>
          <NavLink to="/expenses/new">新規申請</NavLink>
          {(can('approve_department') || can('settle')) && <NavLink to="/approvals">承認待ち</NavLink>}
          {can('view_all_expenses') && <NavLink to="/admin/expenses">全申請</NavLink>}
          {can('manage_masters') && <NavLink to="/admin/masters">マスタ管理</NavLink>}
          {can('view_audit_log') && <NavLink to="/admin/audit">監査ログ</NavLink>}
          {(can('export_reports') || can('run_batch')) && (
            <NavLink to="/admin/operations">帳票・バッチ</NavLink>
          )}
        </nav>
        <div className="user" data-testid="current-user">
          {user.name}（{roleLabel[user.role]} / {user.departmentName}）
          <button type="button" className="btn btn-link" onClick={() => void onLogout()}>
            ログアウト
          </button>
        </div>
      </header>
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  );
}

import type { ReactNode } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import type { Permission } from './api/types';
import { AuthProvider } from './auth/AuthContext';
import { useAuth } from './auth/authContext';
import { Layout } from './components/Layout';
import { AuditLogPage } from './pages/admin/AuditLogPage';
import { MastersPage } from './pages/admin/MastersPage';
import { OperationsPage } from './pages/admin/OperationsPage';
import { ExpenseDetailPage } from './pages/ExpenseDetailPage';
import { ExpenseEditPage } from './pages/ExpenseEditPage';
import { AllExpensesPage, ApprovalsPage, MyExpensesPage } from './pages/ExpenseListPages';
import { ExpenseReportPage } from './pages/ExpenseReportPage';
import { LoginPage } from './pages/LoginPage';

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <p>読み込み中…</p>;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  return <>{children}</>;
}

/** Hides screens the user has no permission for. The API enforces the same rules. */
function RequirePermission({ anyOf, children }: { anyOf: Permission[]; children: ReactNode }) {
  const { can } = useAuth();
  if (!anyOf.some(can)) {
    return (
      <div className="alert alert-error" role="alert">
        この画面を表示する権限がありません。
      </div>
    );
  }
  return <>{children}</>;
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<Navigate to="/expenses" replace />} />
        <Route path="/expenses" element={<MyExpensesPage />} />
        <Route path="/expenses/new" element={<ExpenseEditPage />} />
        <Route path="/expenses/:id" element={<ExpenseDetailPage />} />
        <Route path="/expenses/:id/edit" element={<ExpenseEditPage />} />
        <Route path="/expenses/:id/report" element={<ExpenseReportPage />} />
        <Route
          path="/approvals"
          element={
            <RequirePermission anyOf={['approve_department', 'settle']}>
              <ApprovalsPage />
            </RequirePermission>
          }
        />
        <Route
          path="/admin/expenses"
          element={
            <RequirePermission anyOf={['view_all_expenses']}>
              <AllExpensesPage />
            </RequirePermission>
          }
        />
        <Route
          path="/admin/masters"
          element={
            <RequirePermission anyOf={['manage_masters']}>
              <MastersPage />
            </RequirePermission>
          }
        />
        <Route
          path="/admin/audit"
          element={
            <RequirePermission anyOf={['view_audit_log']}>
              <AuditLogPage />
            </RequirePermission>
          }
        />
        <Route
          path="/admin/operations"
          element={
            <RequirePermission anyOf={['export_reports', 'run_batch']}>
              <OperationsPage />
            </RequirePermission>
          }
        />
        <Route path="*" element={<p>ページが見つかりません。</p>} />
      </Route>
    </Routes>
  );
}

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AuthProvider>
  );
}

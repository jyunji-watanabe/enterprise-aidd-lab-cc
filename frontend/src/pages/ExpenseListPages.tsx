import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../api/api';
import type { Status } from '../api/types';
import { useAuth } from '../auth/authContext';
import { ErrorAlert } from '../components/ErrorAlert';
import { ExpenseTable } from '../components/ExpenseTable';
import { useLoad } from '../components/useLoad';
import { statusLabel } from '../domain/labels';

const statuses = Object.keys(statusLabel) as Status[];

function StatusFilter({ value, onChange }: { value: Status | ''; onChange: (s: Status | '') => void }) {
  return (
    <label className="inline">
      ステータス
      <select value={value} onChange={(e) => onChange(e.target.value as Status | '')} aria-label="ステータス">
        <option value="">すべて</option>
        {statuses.map((s) => (
          <option key={s} value={s}>
            {statusLabel[s]}
          </option>
        ))}
      </select>
    </label>
  );
}

export function MyExpensesPage() {
  const [status, setStatus] = useState<Status | ''>('');
  const { data, error, loading } = useLoad(() => api.expenses('mine', status), `mine-${status}`);
  return (
    <section>
      <div className="page-header">
        <h1>自分の申請</h1>
        <Link className="btn btn-primary" to="/expenses/new">
          新規申請
        </Link>
      </div>
      <StatusFilter value={status} onChange={setStatus} />
      <ErrorAlert error={error} />
      {loading && !data ? <p>読み込み中…</p> : <ExpenseTable expenses={data ?? []} />}
    </section>
  );
}

export function ApprovalsPage() {
  const { can } = useAuth();
  const { data, error, loading } = useLoad(() => api.expenses('approvals'), 'approvals');
  return (
    <section>
      <h1>承認待ち</h1>
      <p className="muted">
        {can('settle') ? '一次承認済みで精算確定待ちの申請です。' : '自部門メンバーから提出された申請です。'}
      </p>
      <ErrorAlert error={error} />
      {loading && !data ? <p>読み込み中…</p> : <ExpenseTable expenses={data ?? []} showApplicant />}
    </section>
  );
}

export function AllExpensesPage() {
  const [status, setStatus] = useState<Status | ''>('');
  const { data, error, loading } = useLoad(() => api.expenses('all', status), `all-${status}`);
  return (
    <section>
      <h1>全申請</h1>
      <StatusFilter value={status} onChange={setStatus} />
      <ErrorAlert error={error} />
      {loading && !data ? <p>読み込み中…</p> : <ExpenseTable expenses={data ?? []} showApplicant />}
    </section>
  );
}

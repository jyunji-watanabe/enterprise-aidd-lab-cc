import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { api } from '../api/api';
import type { Action, ExpenseDetail } from '../api/types';
import { ActionPanel } from '../components/ActionPanel';
import { ErrorAlert } from '../components/ErrorAlert';
import { HistoryTable } from '../components/HistoryTable';
import { ItemsTable } from '../components/ItemsTable';
import { StatusBadge } from '../components/StatusBadge';
import { useLoad } from '../components/useLoad';
import { formatDate, formatDateTime } from '../domain/format';
import { actionLabel } from '../domain/labels';

export function ExpenseDetailPage() {
  const id = Number(useParams().id);
  const { data, error } = useLoad(() => api.expense(id), String(id));
  const [expense, setExpense] = useState<ExpenseDetail>();
  const [notice, setNotice] = useState('');

  useEffect(() => {
    setExpense(data);
  }, [data]);

  if (error) return <ErrorAlert error={error} />;
  if (!expense) return <p>読み込み中…</p>;

  const onAction = async (action: Action, comment: string) => {
    const updated = await api.transition(expense.id, action, comment);
    setExpense(updated);
    setNotice(`${actionLabel[action]}しました。`);
  };

  return (
    <section>
      <div className="page-header">
        <h1>
          <span data-testid="expense-number">{expense.number}</span> <StatusBadge status={expense.status} />
        </h1>
        <div className="links">
          {expense.canEdit && (
            <Link className="btn" to={`/expenses/${expense.id}/edit`}>
              編集
            </Link>
          )}
          <Link className="btn" to={`/expenses/${expense.id}/report`}>
            経費精算書（印刷）
          </Link>
        </div>
      </div>
      {notice && (
        <div className="alert alert-success" role="status">
          {notice}
        </div>
      )}
      <dl className="card meta">
        <dt>件名</dt>
        <dd>{expense.title || '(件名なし)'}</dd>
        <dt>申請者</dt>
        <dd>
          {expense.applicantName}（{expense.applicantId}）
        </dd>
        <dt>部門</dt>
        <dd>{expense.departmentName}</dd>
        <dt>申請日</dt>
        <dd>{formatDate(expense.appliedAt)}</dd>
        <dt>最終更新</dt>
        <dd>{formatDateTime(expense.updatedAt)}</dd>
      </dl>
      <h2>明細</h2>
      <ItemsTable items={expense.items} total={expense.totalAmount} />
      <ActionPanel expense={expense} onAction={onAction} />
      <h2>承認履歴</h2>
      <HistoryTable history={expense.history} />
    </section>
  );
}

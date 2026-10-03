import { Link } from 'react-router-dom';
import type { Expense } from '../api/types';
import { formatDate, formatYen } from '../domain/format';
import { StatusBadge } from './StatusBadge';

export function ExpenseTable({
  expenses,
  showApplicant = false,
}: {
  expenses: Expense[];
  showApplicant?: boolean;
}) {
  if (expenses.length === 0) return <p className="muted">該当する申請はありません。</p>;
  return (
    <table className="table">
      <thead>
        <tr>
          <th>申請番号</th>
          <th>件名</th>
          {showApplicant && <th>申請者</th>}
          {showApplicant && <th>部門</th>}
          <th>申請日</th>
          <th className="num">合計金額</th>
          <th>ステータス</th>
        </tr>
      </thead>
      <tbody>
        {expenses.map((e) => (
          <tr key={e.id}>
            <td>
              <Link to={`/expenses/${e.id}`}>{e.number}</Link>
            </td>
            <td>{e.title || '(件名なし)'}</td>
            {showApplicant && <td>{e.applicantName}</td>}
            {showApplicant && <td>{e.departmentName}</td>}
            <td>{formatDate(e.appliedAt)}</td>
            <td className="num">{formatYen(e.totalAmount)}</td>
            <td>
              <StatusBadge status={e.status} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

import { Link, useParams } from 'react-router-dom';
import { api } from '../api/api';
import type { Action, ExpenseDetail, HistoryEntry } from '../api/types';
import { ErrorAlert } from '../components/ErrorAlert';
import { HistoryTable } from '../components/HistoryTable';
import { ItemsTable } from '../components/ItemsTable';
import { useLoad } from '../components/useLoad';
import { formatDate, formatDateTime, formatYen } from '../domain/format';
import { statusLabel } from '../domain/labels';

/** Latest history entry for the action (after any withdrawals / resubmissions). */
function latest(history: HistoryEntry[], action: Action): HistoryEntry | undefined {
  return [...history].reverse().find((h) => h.action === action);
}

function StampBox({ label, entry }: { label: string; entry: HistoryEntry | undefined }) {
  return (
    <div className="stamp">
      <div className="stamp-label">{label}</div>
      <div className="stamp-body">
        {entry ? (
          <>
            <div className="stamp-name">{entry.actorName}</div>
            <div className="stamp-time">{formatDateTime(entry.createdAt)}</div>
          </>
        ) : (
          <span className="muted">未</span>
        )}
      </div>
    </div>
  );
}

/** 経費精算書（控）: print-optimised layout. Use the browser's print to save as PDF. */
export function ExpenseReportPage() {
  const id = Number(useParams().id);
  const { data, error } = useLoad<ExpenseDetail>(() => api.expense(id), String(id));
  if (error) return <ErrorAlert error={error} />;
  if (!data) return <p>読み込み中…</p>;

  return (
    <div className="report">
      <div className="no-print report-toolbar">
        <Link to={`/expenses/${data.id}`}>← 詳細に戻る</Link>
        <button type="button" className="btn btn-primary" onClick={() => window.print()}>
          印刷 / PDF保存
        </button>
      </div>
      <article className="report-sheet">
        <h1 className="report-title">経費精算書（控）</h1>
        <div className="report-head">
          <table className="report-meta">
            <tbody>
              <tr>
                <th>申請番号</th>
                <td data-testid="report-number">{data.number}</td>
              </tr>
              <tr>
                <th>申請者</th>
                <td>
                  {data.applicantName}（{data.applicantId}）
                </td>
              </tr>
              <tr>
                <th>所属部門</th>
                <td>{data.departmentName}</td>
              </tr>
              <tr>
                <th>件名</th>
                <td>{data.title}</td>
              </tr>
              <tr>
                <th>申請日</th>
                <td>{formatDate(data.appliedAt)}</td>
              </tr>
              <tr>
                <th>ステータス</th>
                <td>{statusLabel[data.status]}</td>
              </tr>
              <tr>
                <th>合計金額</th>
                <td className="report-total">{formatYen(data.totalAmount)}</td>
              </tr>
            </tbody>
          </table>
          <div className="stamps">
            <StampBox label="申請者" entry={latest(data.history, 'submit')} />
            <StampBox label="承認者（部門長）" entry={latest(data.history, 'approve')} />
            <StampBox label="経理（精算確定）" entry={latest(data.history, 'settle')} />
          </div>
        </div>
        <h2>明細一覧</h2>
        <ItemsTable items={data.items} total={data.totalAmount} links={false} />
        <h2>承認履歴</h2>
        <HistoryTable history={data.history} />
        <p className="small muted">出力日時: {formatDateTime(new Date().toISOString())}</p>
      </article>
    </div>
  );
}

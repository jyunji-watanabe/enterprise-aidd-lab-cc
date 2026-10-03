import type { HistoryEntry } from '../api/types';
import { formatDateTime } from '../domain/format';
import { actionLabel, statusLabel } from '../domain/labels';

export function HistoryTable({ history }: { history: HistoryEntry[] }) {
  if (history.length === 0) return <p className="muted">履歴はありません。</p>;
  return (
    <table className="table">
      <thead>
        <tr>
          <th>日時</th>
          <th>操作</th>
          <th>ステータス</th>
          <th>実行者</th>
          <th>コメント</th>
        </tr>
      </thead>
      <tbody>
        {history.map((h) => (
          <tr key={h.id}>
            <td>{formatDateTime(h.createdAt)}</td>
            <td>{actionLabel[h.action]}</td>
            <td>
              {statusLabel[h.fromStatus]} → {statusLabel[h.toStatus]}
            </td>
            <td>{h.actorName}</td>
            <td className="pre">{h.comment}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

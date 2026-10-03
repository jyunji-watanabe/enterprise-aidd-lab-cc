import { useState } from 'react';
import { api } from '../../api/api';
import type { ReminderResult } from '../../api/types';
import { ErrorAlert } from '../../components/ErrorAlert';
import { useLoad } from '../../components/useLoad';
import { formatDateTime, todayISO } from '../../domain/format';

function firstOfMonth(): string {
  return `${todayISO().slice(0, 8)}01`;
}

export function OperationsPage() {
  const [from, setFrom] = useState(firstOfMonth());
  const [to, setTo] = useState(todayISO());
  const [result, setResult] = useState<ReminderResult | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const logs = useLoad(() => api.reminders(), 'reminders');

  const runBatch = async () => {
    setBusy(true);
    setError(null);
    try {
      setResult(await api.runReminders());
      logs.reload();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section>
      <h1>帳票・バッチ</h1>
      <ErrorAlert error={error} />
      <div className="card">
        <h2>会計連携CSV出力（精算確定データ）</h2>
        <p className="muted">
          指定期間に精算確定（SETTLED）となった申請の明細をCSV（UTF-8 BOM付き）で出力します。
        </p>
        <div className="inline-form">
          <label className="inline">
            開始日
            <input type="date" aria-label="開始日" value={from} onChange={(e) => setFrom(e.target.value)} />
          </label>
          <label className="inline">
            終了日
            <input type="date" aria-label="終了日" value={to} onChange={(e) => setTo(e.target.value)} />
          </label>
          <a className="btn btn-primary" href={api.settledCsvUrl(from, to)} download>
            CSVダウンロード
          </a>
        </div>
      </div>

      <div className="card">
        <h2>未承認リマインドバッチ</h2>
        <p className="muted">
          申請中・一次承認済のまま7日以上経過した申請を抽出し、承認待ちの担当者（部門長/経理）へリマインド（ダミーメール）を記録します。
          CLI からは <code>expense remind</code> で実行できます。
        </p>
        <button type="button" className="btn btn-primary" disabled={busy} onClick={() => void runBatch()}>
          リマインドバッチを実行
        </button>
        {result && (
          <div className="alert alert-info" role="status" data-testid="batch-result">
            実行ID {result.runId}: 対象 {result.targets}件 / リマインド {result.reminders.length}件
          </div>
        )}
      </div>

      <h2>リマインド送信ログ</h2>
      <ErrorAlert error={logs.error} />
      <table className="table">
        <thead>
          <tr>
            <th>日時</th>
            <th>実行ID</th>
            <th>申請番号</th>
            <th>宛先</th>
            <th>本文</th>
          </tr>
        </thead>
        <tbody>
          {(logs.data ?? []).map((r) => (
            <tr key={r.id}>
              <td>{formatDateTime(r.createdAt)}</td>
              <td className="small">{r.runId}</td>
              <td>{r.expenseNumber}</td>
              <td>{r.recipientId}</td>
              <td className="small">{r.message}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

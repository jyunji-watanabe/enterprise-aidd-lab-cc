import { useState } from 'react';
import { api } from '../../api/api';
import { ErrorAlert } from '../../components/ErrorAlert';
import { useLoad } from '../../components/useLoad';
import { formatDateTime } from '../../domain/format';

const PAGE = 50;
const actions = [
  'LOGIN_SUCCESS',
  'LOGIN_FAILURE',
  'LOGOUT',
  'EXPENSE_CREATE',
  'EXPENSE_UPDATE',
  'EXPENSE_SUBMIT',
  'EXPENSE_WITHDRAW',
  'EXPENSE_APPROVE',
  'EXPENSE_REJECT',
  'EXPENSE_SETTLE',
  'ACCOUNT_CREATE',
  'ACCOUNT_UPDATE',
  'ACCOUNT_DELETE',
  'DEPARTMENT_CREATE',
  'DEPARTMENT_UPDATE',
  'DEPARTMENT_DELETE',
  'USER_CREATE',
  'USER_UPDATE',
  'USER_DELETE',
  'BATCH_REMINDER_RUN',
  'CSV_EXPORT',
];

export function AuditLogPage() {
  const [action, setAction] = useState('');
  const [actorId, setActorId] = useState('');
  const [offset, setOffset] = useState(0);
  const [verify, setVerify] = useState<string>('');
  const { data, error } = useLoad(
    () => api.auditLogs({ action, actorId, limit: PAGE, offset }),
    `${action}-${actorId}-${offset}`,
  );

  const onVerify = async () => {
    try {
      const v = await api.verifyAudit();
      setVerify(
        v.valid
          ? `改ざんは検出されませんでした（${v.checked}件検証）`
          : `改ざんを検出しました: ID ${v.brokenAt ?? '?'}`,
      );
    } catch {
      setVerify('検証に失敗しました');
    }
  };

  return (
    <section>
      <div className="page-header">
        <h1>監査ログ</h1>
        <button type="button" className="btn" onClick={() => void onVerify()}>
          改ざん検証
        </button>
      </div>
      {verify && (
        <div className="alert alert-info" role="status">
          {verify}
        </div>
      )}
      <div className="card inline-form">
        <select
          aria-label="操作種別"
          value={action}
          onChange={(e) => {
            setAction(e.target.value);
            setOffset(0);
          }}
        >
          <option value="">すべての操作</option>
          {actions.map((a) => (
            <option key={a}>{a}</option>
          ))}
        </select>
        <input
          aria-label="実行ユーザーID"
          placeholder="実行ユーザーID"
          value={actorId}
          onChange={(e) => {
            setActorId(e.target.value);
            setOffset(0);
          }}
        />
      </div>
      <ErrorAlert error={error} />
      <p className="muted">{data?.total ?? 0}件</p>
      <table className="table audit">
        <thead>
          <tr>
            <th>ID</th>
            <th>日時</th>
            <th>実行ユーザー</th>
            <th>操作種別</th>
            <th>対象</th>
            <th>操作前</th>
            <th>操作後</th>
            <th>IP / クライアント</th>
          </tr>
        </thead>
        <tbody>
          {(data?.items ?? []).map((l) => (
            <tr key={l.id}>
              <td>{l.id}</td>
              <td>{formatDateTime(l.occurredAt)}</td>
              <td>{l.actorId}</td>
              <td>{l.action}</td>
              <td>
                {l.resourceType}/{l.resourceId}
              </td>
              <td>
                <JsonCell value={l.before} />
              </td>
              <td>
                <JsonCell value={l.after} />
              </td>
              <td className="small">
                {l.ipAddress}
                <br />
                {l.userAgent}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="form-actions">
        <button
          type="button"
          className="btn"
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE))}
        >
          前へ
        </button>
        <button
          type="button"
          className="btn"
          disabled={offset + PAGE >= (data?.total ?? 0)}
          onClick={() => setOffset(offset + PAGE)}
        >
          次へ
        </button>
      </div>
    </section>
  );
}

function JsonCell({ value }: { value: string | null }) {
  if (!value) return <span className="muted">-</span>;
  return (
    <details>
      <summary>表示</summary>
      <pre className="json">{JSON.stringify(JSON.parse(value) as unknown, null, 2)}</pre>
    </details>
  );
}

import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api } from '../api/api';
import { ApiError } from '../api/client';
import type { Account, ExpenseItemInput } from '../api/types';
import { ErrorAlert } from '../components/ErrorAlert';
import {
  emptyItem,
  itemHints,
  needsLongDescription,
  totalAmount,
  descriptionLength,
} from '../domain/expenseForm';
import { formatYen, todayISO } from '../domain/format';

interface Row extends ExpenseItemInput {
  key: number;
  attachmentName: string | null;
}

let nextKey = 1;
const toRow = (it: ExpenseItemInput, attachmentName: string | null = null): Row => ({
  ...it,
  key: nextKey++,
  attachmentName,
});

export function ExpenseEditPage() {
  const params = useParams();
  const id = params.id ? Number(params.id) : null;
  const navigate = useNavigate();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [title, setTitle] = useState('');
  const [rows, setRows] = useState<Row[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const abort = new AbortController();
    const cancelled = () => abort.signal.aborted;
    const load = async () => {
      const accs = await api.accounts();
      if (cancelled()) return;
      setAccounts(accs);
      if (id === null) {
        setRows([toRow(emptyItem(todayISO(), accs[0]?.id ?? 0))]);
      } else {
        const d = await api.expense(id);
        if (cancelled()) return;
        if (!d.canEdit) {
          void navigate(`/expenses/${id}`, { replace: true });
          return;
        }
        setTitle(d.title);
        setRows(
          d.items.map((it) =>
            toRow(
              {
                useDate: it.useDate,
                accountId: it.accountId,
                amount: it.amount,
                description: it.description,
                attachmentId: it.attachmentId,
              },
              it.attachmentName,
            ),
          ),
        );
      }
      setLoaded(true);
    };
    load().catch((e: unknown) => {
      if (!cancelled()) setError(e);
    });
    return () => {
      abort.abort();
    };
  }, [id, navigate]);

  const update = (key: number, patch: Partial<Row>) => {
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  };

  const upload = async (key: number, file: File | undefined) => {
    if (!file) return;
    try {
      const att = await api.uploadAttachment(file);
      update(key, { attachmentId: att.id, attachmentName: att.filename });
    } catch (e) {
      setError(e);
    }
  };

  const save = async (submit: boolean) => {
    setBusy(true);
    setError(null);
    const input = {
      title,
      items: rows.map(({ useDate, accountId, amount, description, attachmentId }) => ({
        useDate,
        accountId,
        amount,
        description,
        attachmentId,
      })),
    };
    try {
      const saved = id === null ? await api.createExpense(input) : await api.updateExpense(id, input);
      if (submit) {
        try {
          await api.transition(saved.id, 'submit', '');
        } catch (e) {
          // Keep the saved draft and show why submission failed.
          if (id === null) void navigate(`/expenses/${saved.id}/edit`, { replace: true });
          setError(e);
          return;
        }
      }
      void navigate(`/expenses/${saved.id}`);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const hints = itemHints(rows);
  const serverFieldErrors = new Map(
    error instanceof ApiError ? error.fields.map((f) => [f.field, f.message] as const) : [],
  );
  // Inactive accounts already used on a line stay selectable so the value is visible.
  const usableAccounts = (selected: number) => accounts.filter((a) => a.active || a.id === selected);

  if (!loaded && !error) return <p>読み込み中…</p>;

  return (
    <section>
      <div className="page-header">
        <h1>{id === null ? '新規申請' : '申請の編集'}</h1>
        {id !== null && <Link to={`/expenses/${id}`}>詳細に戻る</Link>}
      </div>
      <ErrorAlert error={error} />
      <div className="card">
        <label>
          件名
          <input name="title" value={title} maxLength={100} onChange={(e) => setTitle(e.target.value)} />
        </label>
      </div>

      <table className="table items">
        <thead>
          <tr>
            <th>#</th>
            <th>利用日</th>
            <th>勘定科目</th>
            <th className="num">金額（円）</th>
            <th>摘要（用途）</th>
            <th>領収書</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const descErr =
              serverFieldErrors.get(`items[${i}].description`) ?? hints[`items[${i}].description`];
            const amountErr = serverFieldErrors.get(`items[${i}].amount`);
            const accErr = serverFieldErrors.get(`items[${i}].accountId`);
            return (
              <tr key={r.key} data-testid={`item-row-${i}`}>
                <td>{i + 1}</td>
                <td>
                  <input
                    type="date"
                    aria-label={`利用日${i + 1}`}
                    value={r.useDate}
                    onChange={(e) => update(r.key, { useDate: e.target.value })}
                  />
                </td>
                <td>
                  <select
                    aria-label={`勘定科目${i + 1}`}
                    value={r.accountId}
                    onChange={(e) => update(r.key, { accountId: Number(e.target.value) })}
                  >
                    {usableAccounts(r.accountId).map((a) => (
                      <option key={a.id} value={a.id}>
                        {a.code} {a.name}
                        {a.limitAmount !== null ? `（上限 ${formatYen(a.limitAmount)}）` : ''}
                      </option>
                    ))}
                  </select>
                  {accErr && <div className="field-error">{accErr}</div>}
                </td>
                <td className="num">
                  <input
                    type="number"
                    min={0}
                    step={1}
                    aria-label={`金額${i + 1}`}
                    value={Number.isFinite(r.amount) ? r.amount : ''}
                    onChange={(e) => update(r.key, { amount: Math.trunc(Number(e.target.value)) })}
                  />
                  {amountErr && <div className="field-error">{amountErr}</div>}
                </td>
                <td>
                  <textarea
                    aria-label={`摘要${i + 1}`}
                    rows={needsLongDescription(r) ? 3 : 1}
                    value={r.description}
                    onChange={(e) => update(r.key, { description: e.target.value })}
                  />
                  <div className="small muted">{descriptionLength(r.description)}文字</div>
                  {descErr && <div className="field-error">{descErr}</div>}
                </td>
                <td>
                  {r.attachmentName && <div className="small">{r.attachmentName}</div>}
                  <input
                    type="file"
                    aria-label={`領収書${i + 1}`}
                    onChange={(e) => void upload(r.key, e.target.files?.[0])}
                  />
                </td>
                <td>
                  <button
                    type="button"
                    className="btn btn-link"
                    onClick={() => setRows((rs) => rs.filter((x) => x.key !== r.key))}
                  >
                    削除
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      <div className="row-actions">
        <button
          type="button"
          className="btn"
          onClick={() => setRows((rs) => [...rs, toRow(emptyItem(todayISO(), accounts[0]?.id ?? 0))])}
        >
          明細を追加
        </button>
        <div className="total">
          合計金額: <strong data-testid="total-amount">{formatYen(totalAmount(rows))}</strong>
        </div>
      </div>
      <div className="form-actions">
        <button type="button" className="btn" disabled={busy} onClick={() => void save(false)}>
          下書き保存
        </button>
        <button type="button" className="btn btn-primary" disabled={busy} onClick={() => void save(true)}>
          保存して提出
        </button>
      </div>
    </section>
  );
}

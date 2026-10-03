import { useState, type SyntheticEvent } from 'react';
import { api, type AccountPayload, type UserPayload } from '../../api/api';
import type { Account, Department, Role, User } from '../../api/types';
import { ErrorAlert } from '../../components/ErrorAlert';
import { useLoad } from '../../components/useLoad';
import { formatYen } from '../../domain/format';
import { roleLabel } from '../../domain/labels';

type Tab = 'accounts' | 'departments' | 'users';

export function MastersPage() {
  const [tab, setTab] = useState<Tab>('accounts');
  return (
    <section>
      <h1>マスタ管理</h1>
      <div className="tabs" role="tablist">
        {(
          [
            ['accounts', '勘定科目'],
            ['departments', '部門'],
            ['users', 'ユーザー'],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            role="tab"
            aria-selected={tab === k}
            className={`tab ${tab === k ? 'active' : ''}`}
            onClick={() => setTab(k)}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === 'accounts' && <AccountsMaster />}
      {tab === 'departments' && <DepartmentsMaster />}
      {tab === 'users' && <UsersMaster />}
    </section>
  );
}

function useMutation(reload: () => void) {
  const [error, setError] = useState<unknown>(null);
  const run = async (fn: () => Promise<unknown>): Promise<boolean> => {
    setError(null);
    try {
      await fn();
      reload();
      return true;
    } catch (e) {
      setError(e);
      return false;
    }
  };
  return { error, run };
}

// ---- Accounts ----

const emptyAccount: AccountPayload = { code: '', name: '', active: true, limitAmount: null };

function AccountsMaster() {
  const { data, error, reload } = useLoad(() => api.accounts(true), 'accounts');
  const m = useMutation(reload);
  const [editing, setEditing] = useState<number | null>(null);
  const [form, setForm] = useState<AccountPayload>(emptyAccount);

  const startEdit = (a: Account) => {
    setEditing(a.id);
    setForm({ code: a.code, name: a.name, active: a.active, limitAmount: a.limitAmount });
  };
  const reset = () => {
    setEditing(null);
    setForm(emptyAccount);
  };
  const onSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    const ok = await m.run(() =>
      editing === null ? api.createAccount(form) : api.updateAccount(editing, form),
    );
    if (ok) reset();
  };

  return (
    <div>
      <ErrorAlert error={error ?? m.error} />
      <form className="card inline-form" onSubmit={(e) => void onSubmit(e)} aria-label="勘定科目フォーム">
        <input
          placeholder="科目コード"
          aria-label="科目コード"
          value={form.code}
          onChange={(e) => setForm({ ...form, code: e.target.value })}
        />
        <input
          placeholder="科目名"
          aria-label="科目名"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
        <input
          type="number"
          placeholder="上限金額（任意）"
          aria-label="上限金額"
          value={form.limitAmount ?? ''}
          onChange={(e) =>
            setForm({ ...form, limitAmount: e.target.value === '' ? null : Number(e.target.value) })
          }
        />
        <label className="inline">
          <input
            type="checkbox"
            checked={form.active}
            onChange={(e) => setForm({ ...form, active: e.target.checked })}
          />
          有効
        </label>
        <button type="submit" className="btn btn-primary">
          {editing === null ? '追加' : '更新'}
        </button>
        {editing !== null && (
          <button type="button" className="btn" onClick={reset}>
            キャンセル
          </button>
        )}
      </form>
      <table className="table">
        <thead>
          <tr>
            <th>コード</th>
            <th>科目名</th>
            <th>状態</th>
            <th className="num">上限金額</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {(data ?? []).map((a) => (
            <tr key={a.id}>
              <td>{a.code}</td>
              <td>{a.name}</td>
              <td>{a.active ? '有効' : '無効'}</td>
              <td className="num">{a.limitAmount !== null ? formatYen(a.limitAmount) : '-'}</td>
              <td>
                <button type="button" className="btn btn-link" onClick={() => startEdit(a)}>
                  編集
                </button>
                <button
                  type="button"
                  className="btn btn-link"
                  onClick={() => {
                    if (window.confirm(`${a.name} を削除しますか？`))
                      void m.run(() => api.deleteAccount(a.id));
                  }}
                >
                  削除
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ---- Departments ----

function DepartmentsMaster() {
  const { data, error, reload } = useLoad(() => api.departments(), 'departments');
  const m = useMutation(reload);
  const [editing, setEditing] = useState<number | null>(null);
  const [form, setForm] = useState({ code: '', name: '' });
  const reset = () => {
    setEditing(null);
    setForm({ code: '', name: '' });
  };
  const onSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    const ok = await m.run(() =>
      editing === null ? api.createDepartment(form) : api.updateDepartment(editing, form),
    );
    if (ok) reset();
  };
  return (
    <div>
      <ErrorAlert error={error ?? m.error} />
      <form className="card inline-form" onSubmit={(e) => void onSubmit(e)} aria-label="部門フォーム">
        <input
          placeholder="部門コード"
          aria-label="部門コード"
          value={form.code}
          onChange={(e) => setForm({ ...form, code: e.target.value })}
        />
        <input
          placeholder="部門名"
          aria-label="部門名"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
        <button type="submit" className="btn btn-primary">
          {editing === null ? '追加' : '更新'}
        </button>
        {editing !== null && (
          <button type="button" className="btn" onClick={reset}>
            キャンセル
          </button>
        )}
      </form>
      <table className="table">
        <thead>
          <tr>
            <th>コード</th>
            <th>部門名</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {(data ?? []).map((d: Department) => (
            <tr key={d.id}>
              <td>{d.code}</td>
              <td>{d.name}</td>
              <td>
                <button
                  type="button"
                  className="btn btn-link"
                  onClick={() => {
                    setEditing(d.id);
                    setForm({ code: d.code, name: d.name });
                  }}
                >
                  編集
                </button>
                <button
                  type="button"
                  className="btn btn-link"
                  onClick={() => {
                    if (window.confirm(`${d.name} を削除しますか？`))
                      void m.run(() => api.deleteDepartment(d.id));
                  }}
                >
                  削除
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ---- Users ----

const roles: Role[] = ['Employee', 'Manager', 'Admin'];

function UsersMaster() {
  const { data, error, reload } = useLoad(() => Promise.all([api.users(), api.departments()]), 'users');
  const m = useMutation(reload);
  const [editing, setEditing] = useState<string | null>(null);
  const empty: UserPayload = { id: '', name: '', departmentId: 0, role: 'Employee', password: '' };
  const [form, setForm] = useState<UserPayload>(empty);
  const [users, departments] = data ?? [[], []];

  const reset = () => {
    setEditing(null);
    setForm(empty);
  };
  const onSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    const payload = { ...form, departmentId: form.departmentId || (departments[0]?.id ?? 0) };
    const ok = await m.run(() =>
      editing === null ? api.createUser(payload) : api.updateUser(editing, payload),
    );
    if (ok) reset();
  };

  return (
    <div>
      <ErrorAlert error={error ?? m.error} />
      <form className="card inline-form" onSubmit={(e) => void onSubmit(e)} aria-label="ユーザーフォーム">
        <input
          placeholder="ユーザーID"
          aria-label="ユーザーID"
          value={form.id ?? ''}
          disabled={editing !== null}
          onChange={(e) => setForm({ ...form, id: e.target.value })}
        />
        <input
          placeholder="氏名"
          aria-label="氏名"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
        <select
          aria-label="所属部門"
          value={form.departmentId || (departments[0]?.id ?? 0)}
          onChange={(e) => setForm({ ...form, departmentId: Number(e.target.value) })}
        >
          {departments.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>
        <select
          aria-label="ロール"
          value={form.role}
          onChange={(e) => setForm({ ...form, role: e.target.value as Role })}
        >
          {roles.map((r) => (
            <option key={r} value={r}>
              {roleLabel[r]}
            </option>
          ))}
        </select>
        <input
          type="password"
          placeholder={editing === null ? 'パスワード（8文字以上）' : '新パスワード（変更時のみ）'}
          aria-label="パスワード"
          value={form.password}
          onChange={(e) => setForm({ ...form, password: e.target.value })}
        />
        <button type="submit" className="btn btn-primary">
          {editing === null ? '追加' : '更新'}
        </button>
        {editing !== null && (
          <button type="button" className="btn" onClick={reset}>
            キャンセル
          </button>
        )}
      </form>
      <table className="table">
        <thead>
          <tr>
            <th>ユーザーID</th>
            <th>氏名</th>
            <th>部門</th>
            <th>ロール</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {users.map((u: User) => (
            <tr key={u.id}>
              <td>{u.id}</td>
              <td>{u.name}</td>
              <td>{u.departmentName}</td>
              <td>{roleLabel[u.role]}</td>
              <td>
                <button
                  type="button"
                  className="btn btn-link"
                  onClick={() => {
                    setEditing(u.id);
                    setForm({
                      id: u.id,
                      name: u.name,
                      departmentId: u.departmentId,
                      role: u.role,
                      password: '',
                    });
                  }}
                >
                  編集
                </button>
                <button
                  type="button"
                  className="btn btn-link"
                  onClick={() => {
                    if (window.confirm(`${u.name} を削除しますか？`)) void m.run(() => api.deleteUser(u.id));
                  }}
                >
                  削除
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

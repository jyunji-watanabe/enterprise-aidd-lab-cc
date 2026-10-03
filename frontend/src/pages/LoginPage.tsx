import { useState, type SyntheticEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/authContext';
import { ErrorAlert } from '../components/ErrorAlert';

export function LoginPage() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [userId, setUserId] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to="/expenses" replace />;

  const onSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(userId, password);
      void navigate('/expenses');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="login">
      <form className="card login-card" onSubmit={(e) => void onSubmit(e)}>
        <h1>経費精算システム</h1>
        <ErrorAlert error={error} />
        <label>
          ユーザーID
          <input
            name="userId"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            autoComplete="username"
            required
          />
        </label>
        <label>
          パスワード
          <input
            name="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        <button type="submit" className="btn btn-primary" disabled={busy}>
          ログイン
        </button>
        <p className="muted small">
          テストアカウント: employee1 / manager1 / admin1（パスワード: Passw0rd!）
        </p>
      </form>
    </div>
  );
}

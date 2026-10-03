import { useState } from 'react';
import type { Action, ExpenseDetail } from '../api/types';
import { actionLabel, commentRequired } from '../domain/labels';
import { ErrorAlert } from './ErrorAlert';

interface Props {
  expense: ExpenseDetail;
  onAction: (action: Action, comment: string) => Promise<void>;
}

/**
 * Renders exactly the actions the backend reports as allowed for the current
 * user (`allowedActions`). No status/role logic lives here.
 */
export function ActionPanel({ expense, onAction }: Props) {
  const [comment, setComment] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const actions = expense.allowedActions;
  if (actions.length === 0) return null;
  const needsComment = actions.some(commentRequired);

  const run = async (a: Action) => {
    if (a === 'withdraw' && !window.confirm('申請を取り下げて下書きに戻しますか？')) return;
    setBusy(true);
    setError(null);
    try {
      await onAction(a, comment);
      setComment('');
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card no-print" data-testid="action-panel">
      <h2>操作</h2>
      <ErrorAlert error={error} />
      {needsComment && (
        <label>
          コメント（承認・差戻し時は必須）
          <textarea name="comment" rows={3} value={comment} onChange={(e) => setComment(e.target.value)} />
        </label>
      )}
      <div className="form-actions">
        {actions.map((a) => (
          <button
            key={a}
            type="button"
            className={`btn ${a === 'reject' || a === 'withdraw' ? 'btn-danger' : 'btn-primary'}`}
            disabled={busy || (commentRequired(a) && comment.trim() === '')}
            onClick={() => void run(a)}
          >
            {actionLabel[a]}
          </button>
        ))}
      </div>
    </div>
  );
}

import { ApiError, errorMessage } from '../api/client';

export function ErrorAlert({ error }: { error: unknown }) {
  if (!error) return null;
  const fields = error instanceof ApiError ? error.fields : [];
  return (
    <div className="alert alert-error" role="alert">
      <div>{errorMessage(error)}</div>
      {fields.length > 0 && (
        <ul>
          {fields.map((f) => (
            <li key={f.field + f.message}>
              <code>{f.field}</code>: {f.message}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

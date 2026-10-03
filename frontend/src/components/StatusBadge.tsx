import type { Status } from '../api/types';
import { statusLabel } from '../domain/labels';

export function StatusBadge({ status }: { status: Status }) {
  return (
    <span className={`badge badge-${status.toLowerCase()}`} data-status={status}>
      {statusLabel[status]}
    </span>
  );
}

// Display labels only. Business decisions (which actions are allowed, who may
// see what) are made by the backend and delivered via `allowedActions`/`canEdit`.
import type { Action, Role, Status } from '../api/types';

export const statusLabel: Record<Status, string> = {
  DRAFT: '下書き',
  SUBMITTED: '申請中',
  APPROVED_BY_MGR: '一次承認済',
  SETTLED: '精算確定',
  REJECTED: '差戻し',
};

export const actionLabel: Record<Action, string> = {
  submit: '提出',
  withdraw: '取下げ',
  approve: '承認',
  reject: '差戻し',
  settle: '精算確定',
};

export const roleLabel: Record<Role, string> = {
  Employee: '一般社員',
  Manager: '承認者/部門長',
  Admin: '経理/管理者',
};

/** Actions that require a comment; mirrors the backend rule for UX only. */
export const commentRequired = (a: Action): boolean => a === 'approve' || a === 'reject' || a === 'settle';

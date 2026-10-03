// Types mirroring the backend JSON API.

export type Role = 'Employee' | 'Manager' | 'Admin';
export type Status = 'DRAFT' | 'SUBMITTED' | 'APPROVED_BY_MGR' | 'SETTLED' | 'REJECTED';
export type Action = 'submit' | 'withdraw' | 'approve' | 'reject' | 'settle';
export type Permission =
  | 'manage_masters'
  | 'view_all_expenses'
  | 'view_audit_log'
  | 'export_reports'
  | 'run_batch'
  | 'approve_department'
  | 'settle';

export interface User {
  id: string;
  name: string;
  departmentId: number;
  departmentName: string;
  role: Role;
  createdAt: string;
  updatedAt: string;
}

export interface Me extends User {
  permissions: Permission[];
}

export interface Department {
  id: number;
  code: string;
  name: string;
}

export interface Account {
  id: number;
  code: string;
  name: string;
  active: boolean;
  limitAmount: number | null;
}

export interface Expense {
  id: number;
  number: string;
  applicantId: string;
  applicantName: string;
  departmentId: number;
  departmentCode: string;
  departmentName: string;
  title: string;
  status: Status;
  totalAmount: number;
  appliedAt: string | null;
  settledAt: string | null;
  statusChangedAt: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExpenseItem {
  id: number;
  lineNo: number;
  useDate: string;
  accountId: number;
  accountCode: string;
  accountName: string;
  amount: number;
  description: string;
  attachmentId: number | null;
  attachmentName: string | null;
}

export interface HistoryEntry {
  id: number;
  action: Action;
  fromStatus: Status;
  toStatus: Status;
  actorId: string;
  actorName: string;
  comment: string;
  createdAt: string;
}

export interface ExpenseDetail extends Expense {
  items: ExpenseItem[];
  history: HistoryEntry[];
  allowedActions: Action[];
  canEdit: boolean;
}

export interface ExpenseItemInput {
  useDate: string;
  accountId: number;
  amount: number;
  description: string;
  attachmentId: number | null;
}

export interface ExpenseInput {
  title: string;
  items: ExpenseItemInput[];
}

export interface Attachment {
  id: number;
  filename: string;
  contentType: string;
  size: number;
}

export interface AuditLog {
  id: number;
  occurredAt: string;
  actorId: string;
  action: string;
  resourceType: string;
  resourceId: string;
  before: string | null;
  after: string | null;
  ipAddress: string;
  userAgent: string;
  hash: string;
}

export interface ReminderLog {
  id: number;
  runId: string;
  expenseId: number;
  expenseNumber: string;
  recipientId: string;
  message: string;
  createdAt: string;
}

export interface ReminderResult {
  runId: string;
  threshold: string;
  targets: number;
  reminders: ReminderLog[];
}

export interface FieldError {
  field: string;
  message: string;
}

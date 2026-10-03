-- Initial schema for the expense management system.
-- All timestamps are stored as UTC RFC3339 strings (e.g. 2026-10-03T09:00:00Z).

CREATE TABLE departments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    code        TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE accounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    code          TEXT    NOT NULL UNIQUE,
    name          TEXT    NOT NULL,
    active        INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    limit_amount  INTEGER NULL CHECK (limit_amount IS NULL OR limit_amount > 0),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);

CREATE TABLE users (
    id             TEXT    PRIMARY KEY,
    name           TEXT    NOT NULL,
    department_id  INTEGER NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    role           TEXT    NOT NULL CHECK (role IN ('Employee', 'Manager', 'Admin')),
    password_hash  TEXT    NOT NULL,
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL
);

CREATE TABLE sessions (
    token_hash  TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE attachments (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    filename      TEXT    NOT NULL,
    content_type  TEXT    NOT NULL,
    size          INTEGER NOT NULL,
    data          BLOB    NOT NULL,
    uploaded_by   TEXT    NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TEXT    NOT NULL
);

CREATE TABLE expenses (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    number             TEXT    UNIQUE,
    applicant_id       TEXT    NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    department_id      INTEGER NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    title              TEXT    NOT NULL DEFAULT '',
    status             TEXT    NOT NULL CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED_BY_MGR', 'SETTLED', 'REJECTED')),
    total_amount       INTEGER NOT NULL DEFAULT 0,
    applied_at         TEXT    NULL,
    settled_at         TEXT    NULL,
    status_changed_at  TEXT    NOT NULL,
    created_at         TEXT    NOT NULL,
    updated_at         TEXT    NOT NULL
);
CREATE INDEX idx_expenses_applicant ON expenses(applicant_id);
CREATE INDEX idx_expenses_status ON expenses(status, department_id);

CREATE TABLE expense_items (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    expense_id     INTEGER NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    line_no        INTEGER NOT NULL,
    use_date       TEXT    NOT NULL,
    account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount         INTEGER NOT NULL,
    description    TEXT    NOT NULL DEFAULT '',
    attachment_id  INTEGER NULL REFERENCES attachments(id) ON DELETE SET NULL,
    UNIQUE (expense_id, line_no)
);

-- Approval history (shown on the expense report).
CREATE TABLE expense_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    expense_id   INTEGER NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    action       TEXT    NOT NULL,
    from_status  TEXT    NOT NULL,
    to_status    TEXT    NOT NULL,
    actor_id     TEXT    NOT NULL,
    actor_name   TEXT    NOT NULL,
    comment      TEXT    NOT NULL DEFAULT '',
    created_at   TEXT    NOT NULL
);
CREATE INDEX idx_expense_history_expense ON expense_history(expense_id);

-- Tamper-resistant audit log: append-only (enforced by triggers) and
-- hash-chained (each row's hash covers its content and the previous hash).
CREATE TABLE audit_logs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at    TEXT NOT NULL,
    actor_id       TEXT NOT NULL,
    action         TEXT NOT NULL,
    resource_type  TEXT NOT NULL,
    resource_id    TEXT NOT NULL,
    before_json    TEXT NULL,
    after_json     TEXT NULL,
    ip_address     TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT '',
    prev_hash      TEXT NOT NULL,
    hash           TEXT NOT NULL
);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

CREATE TRIGGER audit_logs_no_update BEFORE UPDATE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs is append-only');
END;

CREATE TRIGGER audit_logs_no_delete BEFORE DELETE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs is append-only');
END;

-- Output of the reminder batch (dummy e-mail log).
CREATE TABLE reminder_logs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        TEXT    NOT NULL,
    expense_id    INTEGER NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    recipient_id  TEXT    NOT NULL,
    message       TEXT    NOT NULL,
    created_at    TEXT    NOT NULL
);
CREATE INDEX idx_reminder_logs_run ON reminder_logs(run_id);

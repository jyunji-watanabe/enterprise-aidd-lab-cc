package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
)

// Expense is an expense claim header.
type Expense struct {
	ID              int64         `json:"id"`
	Number          string        `json:"number"`
	ApplicantID     string        `json:"applicantId"`
	ApplicantName   string        `json:"applicantName"`
	DepartmentID    int64         `json:"departmentId"`
	DepartmentCode  string        `json:"departmentCode"`
	DepartmentName  string        `json:"departmentName"`
	Title           string        `json:"title"`
	Status          domain.Status `json:"status"`
	TotalAmount     int64         `json:"totalAmount"`
	AppliedAt       *string       `json:"appliedAt"`
	SettledAt       *string       `json:"settledAt"`
	StatusChangedAt string        `json:"statusChangedAt"`
	CreatedAt       string        `json:"createdAt"`
	UpdatedAt       string        `json:"updatedAt"`
}

// Ref returns the authorization view of the expense.
func (e Expense) Ref() domain.ExpenseRef {
	return domain.ExpenseRef{ApplicantID: e.ApplicantID, DepartmentID: e.DepartmentID, Status: e.Status}
}

// ExpenseItem is a stored expense line, enriched with master names.
type ExpenseItem struct {
	ID             int64   `json:"id"`
	LineNo         int     `json:"lineNo"`
	UseDate        string  `json:"useDate"`
	AccountID      int64   `json:"accountId"`
	AccountCode    string  `json:"accountCode"`
	AccountName    string  `json:"accountName"`
	Amount         int64   `json:"amount"`
	Description    string  `json:"description"`
	AttachmentID   *int64  `json:"attachmentId"`
	AttachmentName *string `json:"attachmentName"`
}

// HistoryEntry is one approval-flow event on an expense.
type HistoryEntry struct {
	ID         int64         `json:"id"`
	Action     domain.Action `json:"action"`
	FromStatus domain.Status `json:"fromStatus"`
	ToStatus   domain.Status `json:"toStatus"`
	ActorID    string        `json:"actorId"`
	ActorName  string        `json:"actorName"`
	Comment    string        `json:"comment"`
	CreatedAt  string        `json:"createdAt"`
}

const expenseSelect = `SELECT e.id, COALESCE(e.number, ''), e.applicant_id, u.name, e.department_id, d.code, d.name, e.title,
 e.status, e.total_amount, e.applied_at, e.settled_at, e.status_changed_at, e.created_at, e.updated_at
FROM expenses e JOIN users u ON u.id = e.applicant_id JOIN departments d ON d.id = e.department_id`

func scanExpense(s interface{ Scan(...any) error }) (Expense, error) {
	var e Expense
	err := s.Scan(&e.ID, &e.Number, &e.ApplicantID, &e.ApplicantName, &e.DepartmentID, &e.DepartmentCode, &e.DepartmentName, &e.Title,
		&e.Status, &e.TotalAmount, &e.AppliedAt, &e.SettledAt, &e.StatusChangedAt, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// ExpenseFilter narrows ListExpenses. Zero values mean "no filter".
type ExpenseFilter struct {
	ApplicantID      string
	DepartmentID     int64
	ExcludeApplicant string
	Statuses         []domain.Status
	SettledFrom      string // inclusive, RFC3339
	SettledTo        string // exclusive, RFC3339
	StatusBefore     string // status_changed_at <= value
}

// ListExpenses returns expenses matching the filter, newest first.
func ListExpenses(ctx context.Context, q DBTX, f ExpenseFilter) ([]Expense, error) {
	var where []string
	var args []any
	if f.ApplicantID != "" {
		where = append(where, "e.applicant_id = ?")
		args = append(args, f.ApplicantID)
	}
	if f.ExcludeApplicant != "" {
		where = append(where, "e.applicant_id <> ?")
		args = append(args, f.ExcludeApplicant)
	}
	if f.DepartmentID > 0 {
		where = append(where, "e.department_id = ?")
		args = append(args, f.DepartmentID)
	}
	if len(f.Statuses) > 0 {
		ph := make([]string, len(f.Statuses))
		for i, s := range f.Statuses {
			ph[i] = "?"
			args = append(args, s)
		}
		where = append(where, "e.status IN ("+strings.Join(ph, ",")+")")
	}
	if f.SettledFrom != "" {
		where = append(where, "e.settled_at >= ?")
		args = append(args, f.SettledFrom)
	}
	if f.SettledTo != "" {
		where = append(where, "e.settled_at < ?")
		args = append(args, f.SettledTo)
	}
	if f.StatusBefore != "" {
		where = append(where, "e.status_changed_at <= ?")
		args = append(args, f.StatusBefore)
	}
	query := expenseSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY e.id DESC"
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Expense{}
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetExpense loads an expense header.
func GetExpense(ctx context.Context, q DBTX, id int64) (Expense, error) {
	e, err := scanExpense(q.QueryRowContext(ctx, expenseSelect+` WHERE e.id = ?`, id))
	return e, notFound(err)
}

// InsertExpense creates a draft expense and assigns its number. Returns the ID.
func InsertExpense(ctx context.Context, q DBTX, applicantID string, deptID int64, title string, total int64, now string) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO expenses (applicant_id, department_id, title, status, total_amount, status_changed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, applicantID, deptID, title, domain.StatusDraft, total, now, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	number := fmt.Sprintf("EXP-%s-%06d", now[:4], id)
	if _, err := q.ExecContext(ctx, `UPDATE expenses SET number = ? WHERE id = ?`, number, id); err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateExpenseContent updates title/total of a draft.
func UpdateExpenseContent(ctx context.Context, q DBTX, id int64, title string, total int64, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE expenses SET title = ?, total_amount = ?, updated_at = ? WHERE id = ? AND status = ?`,
		title, total, now, id, domain.StatusDraft)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrConflict
	}
	return nil
}

// UpdateExpenseStatus moves an expense from one status to another. It uses a
// compare-and-set on the current status so concurrent changes are detected.
func UpdateExpenseStatus(ctx context.Context, q DBTX, id int64, from, to domain.Status, now string) error {
	sets := "status = ?, status_changed_at = ?, updated_at = ?"
	args := []any{to, now, now}
	switch to {
	case domain.StatusSubmitted:
		sets += ", applied_at = ?"
		args = append(args, now)
	case domain.StatusSettled:
		sets += ", settled_at = ?"
		args = append(args, now)
	case domain.StatusDraft:
		sets += ", applied_at = NULL"
	}
	args = append(args, id, from)
	res, err := q.ExecContext(ctx, `UPDATE expenses SET `+sets+` WHERE id = ? AND status = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrConflict
	}
	return nil
}

// ListExpenseItems returns the lines of an expense in order.
func ListExpenseItems(ctx context.Context, q DBTX, expenseID int64) ([]ExpenseItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT i.id, i.line_no, i.use_date, i.account_id, a.code, a.name, i.amount, i.description, i.attachment_id, att.filename
FROM expense_items i JOIN accounts a ON a.id = i.account_id LEFT JOIN attachments att ON att.id = i.attachment_id
WHERE i.expense_id = ? ORDER BY i.line_no`, expenseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ExpenseItem{}
	for rows.Next() {
		var it ExpenseItem
		if err := rows.Scan(&it.ID, &it.LineNo, &it.UseDate, &it.AccountID, &it.AccountCode, &it.AccountName, &it.Amount, &it.Description, &it.AttachmentID, &it.AttachmentName); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReplaceExpenseItems deletes and re-inserts all lines of an expense.
func ReplaceExpenseItems(ctx context.Context, q DBTX, expenseID int64, items []domain.ExpenseItem) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM expense_items WHERE expense_id = ?`, expenseID); err != nil {
		return err
	}
	for i, it := range items {
		_, err := q.ExecContext(ctx, `INSERT INTO expense_items (expense_id, line_no, use_date, account_id, amount, description, attachment_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			expenseID, i+1, it.UseDate, it.AccountID, it.Amount, it.Description, it.AttachmentID)
		if err != nil {
			return err
		}
	}
	return nil
}

// InsertHistory records an approval-flow event.
func InsertHistory(ctx context.Context, q DBTX, expenseID int64, h HistoryEntry) error {
	_, err := q.ExecContext(ctx, `INSERT INTO expense_history (expense_id, action, from_status, to_status, actor_id, actor_name, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		expenseID, h.Action, h.FromStatus, h.ToStatus, h.ActorID, h.ActorName, h.Comment, h.CreatedAt)
	return err
}

// ListHistory returns the approval-flow events of an expense in order.
func ListHistory(ctx context.Context, q DBTX, expenseID int64) ([]HistoryEntry, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, action, from_status, to_status, actor_id, actor_name, comment, created_at FROM expense_history WHERE expense_id = ? ORDER BY id`, expenseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []HistoryEntry{}
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.ID, &h.Action, &h.FromStatus, &h.ToStatus, &h.ActorID, &h.ActorName, &h.Comment, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

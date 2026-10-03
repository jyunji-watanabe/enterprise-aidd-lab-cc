package store

import (
	"context"
)

// Attachment is an uploaded receipt file.
type Attachment struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Data        []byte `json:"-"`
	UploadedBy  string `json:"uploadedBy"`
	CreatedAt   string `json:"createdAt"`
}

// InsertAttachment stores a file and returns its ID.
func InsertAttachment(ctx context.Context, q DBTX, a Attachment) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO attachments (filename, content_type, size, data, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.Filename, a.ContentType, a.Size, a.Data, a.UploadedBy, a.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetAttachment loads a file including its data.
func GetAttachment(ctx context.Context, q DBTX, id int64) (Attachment, error) {
	var a Attachment
	err := q.QueryRowContext(ctx, `SELECT id, filename, content_type, size, data, uploaded_by, created_at FROM attachments WHERE id = ?`, id).
		Scan(&a.ID, &a.Filename, &a.ContentType, &a.Size, &a.Data, &a.UploadedBy, &a.CreatedAt)
	return a, notFound(err)
}

// ExpenseIDsForAttachment returns the expenses whose lines reference the attachment.
func ExpenseIDsForAttachment(ctx context.Context, q DBTX, attachmentID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT expense_id FROM expense_items WHERE attachment_id = ?`, attachmentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ReminderLog is a dummy e-mail reminder produced by the batch.
type ReminderLog struct {
	ID            int64  `json:"id"`
	RunID         string `json:"runId"`
	ExpenseID     int64  `json:"expenseId"`
	ExpenseNumber string `json:"expenseNumber"`
	RecipientID   string `json:"recipientId"`
	Message       string `json:"message"`
	CreatedAt     string `json:"createdAt"`
}

// InsertReminder stores a reminder log entry.
func InsertReminder(ctx context.Context, q DBTX, r ReminderLog) error {
	_, err := q.ExecContext(ctx, `INSERT INTO reminder_logs (run_id, expense_id, recipient_id, message, created_at) VALUES (?, ?, ?, ?, ?)`,
		r.RunID, r.ExpenseID, r.RecipientID, r.Message, r.CreatedAt)
	return err
}

// ListReminders returns the most recent reminder logs.
func ListReminders(ctx context.Context, q DBTX, limit int) ([]ReminderLog, error) {
	rows, err := q.QueryContext(ctx, `SELECT r.id, r.run_id, r.expense_id, COALESCE(e.number, ''), r.recipient_id, r.message, r.created_at
FROM reminder_logs r JOIN expenses e ON e.id = r.expense_id ORDER BY r.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ReminderLog{}
	for rows.Next() {
		var r ReminderLog
		if err := rows.Scan(&r.ID, &r.RunID, &r.ExpenseID, &r.ExpenseNumber, &r.RecipientID, &r.Message, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountUsers returns the number of users (used to decide whether to seed).
func CountUsers(ctx context.Context, q DBTX) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

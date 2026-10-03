package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

// MaxAttachmentSize limits uploaded receipt files.
const MaxAttachmentSize = 5 << 20

// UploadAttachment stores a receipt file owned by the actor.
func (s *Service) UploadAttachment(ctx context.Context, actor domain.Actor, filename, contentType string, data []byte) (store.Attachment, error) {
	if len(data) == 0 {
		return store.Attachment{}, badRequest("ファイルが空です")
	}
	if len(data) > MaxAttachmentSize {
		return store.Attachment{}, badRequest("ファイルサイズは5MBまでです")
	}
	name := filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		name = "receipt"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	a := store.Attachment{Filename: name, ContentType: contentType, Size: int64(len(data)), Data: data, UploadedBy: actor.UserID, CreatedAt: s.now()}
	id, err := store.InsertAttachment(ctx, s.Store.DB, a)
	if err != nil {
		return store.Attachment{}, err
	}
	a.ID = id
	return a, nil
}

// GetAttachment returns a file if the actor uploaded it or may view an expense that references it.
func (s *Service) GetAttachment(ctx context.Context, actor domain.Actor, id int64) (store.Attachment, error) {
	a, err := store.GetAttachment(ctx, s.Store.DB, id)
	if err != nil {
		return store.Attachment{}, err
	}
	if a.UploadedBy == actor.UserID {
		return a, nil
	}
	ids, err := store.ExpenseIDsForAttachment(ctx, s.Store.DB, id)
	if err != nil {
		return store.Attachment{}, err
	}
	for _, eid := range ids {
		e, err := store.GetExpense(ctx, s.Store.DB, eid)
		if err != nil {
			continue
		}
		if domain.CanView(actor, e.Ref()) {
			return a, nil
		}
	}
	return store.Attachment{}, domain.ErrForbidden
}

// parseDateRange converts inclusive local dates into a UTC [from, to) range.
func (s *Service) parseDateRange(from, to string) (time.Time, time.Time, error) {
	v := &domain.ValidationError{}
	f, err := time.ParseInLocation(domain.DateLayout, from, s.Location)
	if err != nil {
		v.Add("from", "開始日はYYYY-MM-DD形式で指定してください")
	}
	t, err := time.ParseInLocation(domain.DateLayout, to, s.Location)
	if err != nil {
		v.Add("to", "終了日はYYYY-MM-DD形式で指定してください")
	}
	if len(v.Errors) == 0 && t.Before(f) {
		v.Add("to", "終了日は開始日以降を指定してください")
	}
	if err := v.OrNil(); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return f, t.AddDate(0, 0, 1), nil
}

// utf8BOM marks the CSV as UTF-8 for spreadsheet software.
const utf8BOM = "\xEF\xBB\xBF"

// CSVHeader is the column layout of the accounting export.
var CSVHeader = []string{"精算日", "申請番号", "行番号", "申請者ID", "申請者名", "部門コード", "部門名", "科目コード", "科目名", "利用日", "金額", "摘要"}

// ExportSettledCSV writes all SETTLED expense lines whose settlement date
// falls within [from, to] (inclusive, business timezone) as CSV for the
// accounting system. The output is UTF-8 with BOM so it opens cleanly in Excel.
func (s *Service) ExportSettledCSV(ctx context.Context, actor domain.Actor, from, to string, meta Meta, w io.Writer) (int, error) {
	if err := actor.Require(domain.PermExportReports); err != nil {
		return 0, err
	}
	f, t, err := s.parseDateRange(from, to)
	if err != nil {
		return 0, err
	}
	expenses, err := store.ListExpenses(ctx, s.Store.DB, store.ExpenseFilter{
		Statuses:    []domain.Status{domain.StatusSettled},
		SettledFrom: store.FormatTime(f),
		SettledTo:   store.FormatTime(t),
	})
	if err != nil {
		return 0, err
	}
	var rows [][]string
	for i := len(expenses) - 1; i >= 0; i-- { // oldest first
		e := expenses[i]
		items, err := store.ListExpenseItems(ctx, s.Store.DB, e.ID)
		if err != nil {
			return 0, err
		}
		settled := ""
		if e.SettledAt != nil {
			settled = store.ParseTime(*e.SettledAt).In(s.Location).Format(domain.DateLayout)
		}
		for _, it := range items {
			rows = append(rows, []string{settled, e.Number, strconv.Itoa(it.LineNo), e.ApplicantID, e.ApplicantName,
				e.DepartmentCode, e.DepartmentName, it.AccountCode, it.AccountName, it.UseDate,
				strconv.FormatInt(it.Amount, 10), csvSafe(it.Description)})
		}
	}
	err = s.Store.WithTx(ctx, func(tx store.DBTX) error {
		return s.audit(ctx, tx, meta, actor.UserID, AuditCSVExport, "report", "settled_csv", nil,
			map[string]any{"from": from, "to": to, "expenses": len(expenses), "rows": len(rows)})
	})
	if err != nil {
		return 0, err
	}
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return 0, err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(CSVHeader); err != nil {
		return 0, err
	}
	if err := cw.WriteAll(rows); err != nil {
		return 0, err
	}
	return len(rows), cw.Error()
}

// csvSafe neutralises spreadsheet formula injection in free-text cells.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// ---- Batch ----

// ReminderResult summarises one reminder batch run.
type ReminderResult struct {
	RunID     string              `json:"runId"`
	Threshold string              `json:"threshold"`
	Targets   int                 `json:"targets"`
	Reminders []store.ReminderLog `json:"reminders"`
}

// ReminderAge is how long an expense may wait for approval before reminders are sent.
const ReminderAge = 7 * 24 * time.Hour

// RunReminders finds SUBMITTED / APPROVED_BY_MGR expenses that have not
// moved for ReminderAge or longer and records a dummy reminder e-mail to
// each pending approver (department managers or admins respectively).
// It can be triggered from the CLI (triggeredBy="cli") or by an admin.
func (s *Service) RunReminders(ctx context.Context, actor domain.Actor, meta Meta) (ReminderResult, error) {
	if err := actor.Require(domain.PermRunBatch); err != nil {
		return ReminderResult{}, err
	}
	now := s.Now()
	res := ReminderResult{
		RunID:     fmt.Sprintf("remind-%s", now.UTC().Format("20060102T150405Z")),
		Threshold: store.FormatTime(now.Add(-ReminderAge)),
		Reminders: []store.ReminderLog{},
	}
	var mails []string
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		pending, err := store.ListExpenses(ctx, tx, store.ExpenseFilter{
			Statuses:     []domain.Status{domain.StatusSubmitted, domain.StatusApprovedByMgr},
			StatusBefore: res.Threshold,
		})
		if err != nil {
			return err
		}
		res.Targets = len(pending)
		for _, e := range pending {
			recipients, err := pendingApprovers(ctx, tx, e)
			if err != nil {
				return err
			}
			days := int(now.Sub(store.ParseTime(e.StatusChangedAt)).Hours() / 24)
			for _, r := range recipients {
				msg := fmt.Sprintf("[経費精算リマインド] %s 様: 申請 %s（申請者: %s, 金額: %d円, ステータス: %s）が%d日間処理待ちです。",
					r.Name, e.Number, e.ApplicantName, e.TotalAmount, e.Status, days)
				log := store.ReminderLog{RunID: res.RunID, ExpenseID: e.ID, ExpenseNumber: e.Number, RecipientID: r.ID, Message: msg, CreatedAt: store.FormatTime(now)}
				if err := store.InsertReminder(ctx, tx, log); err != nil {
					return err
				}
				res.Reminders = append(res.Reminders, log)
				mails = append(mails, fmt.Sprintf("[DUMMY MAIL] to=%s subject=\"承認待ちリマインド %s\" body=%q\n", r.ID, e.Number, msg))
			}
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditBatchReminder, "batch", res.RunID, nil,
			map[string]any{"threshold": res.Threshold, "targets": res.Targets, "reminders": len(res.Reminders)})
	})
	if err != nil {
		return ReminderResult{}, err
	}
	if s.Mailer != nil {
		for _, m := range mails {
			if _, err := io.WriteString(s.Mailer, m); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

func pendingApprovers(ctx context.Context, tx store.DBTX, e store.Expense) ([]store.User, error) {
	var candidates []store.User
	var err error
	if e.Status == domain.StatusSubmitted {
		candidates, err = store.ListUsersByRole(ctx, tx, domain.RoleManager, e.DepartmentID)
	} else {
		candidates, err = store.ListUsersByRole(ctx, tx, domain.RoleAdmin, 0)
	}
	if err != nil {
		return nil, err
	}
	out := make([]store.User, 0, len(candidates))
	for _, u := range candidates {
		if u.ID != e.ApplicantID {
			out = append(out, u)
		}
	}
	return out, nil
}

// ListReminders returns recent reminder logs (Admin only).
func (s *Service) ListReminders(ctx context.Context, actor domain.Actor) ([]store.ReminderLog, error) {
	if err := actor.Require(domain.PermRunBatch); err != nil {
		return nil, err
	}
	return store.ListReminders(ctx, s.Store.DB, 200)
}

// ---- Audit log ----

// ListAudit returns audit records (Admin only).
func (s *Service) ListAudit(ctx context.Context, actor domain.Actor, f store.AuditFilter) ([]store.AuditLog, int, error) {
	if err := actor.Require(domain.PermViewAuditLog); err != nil {
		return nil, 0, err
	}
	return store.ListAudit(ctx, s.Store.DB, f)
}

// AuditVerification is the result of checking the audit hash chain.
type AuditVerification struct {
	Valid    bool  `json:"valid"`
	Checked  int   `json:"checked"`
	BrokenAt int64 `json:"brokenAt,omitempty"`
}

// VerifyAudit recomputes the audit hash chain (Admin only).
func (s *Service) VerifyAudit(ctx context.Context, actor domain.Actor) (AuditVerification, error) {
	if err := actor.Require(domain.PermViewAuditLog); err != nil {
		return AuditVerification{}, err
	}
	n, broken, err := store.VerifyAuditChain(ctx, s.Store.DB)
	if err != nil {
		return AuditVerification{}, err
	}
	return AuditVerification{Valid: broken == 0, Checked: n, BrokenAt: broken}, nil
}

// SystemActor is used by the CLI batch and seeding; it has the Admin role.
var SystemActor = domain.Actor{UserID: "system", Name: "System", Role: domain.RoleAdmin}

// IsNotFound is a helper for callers outside the package.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

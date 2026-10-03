package service_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/seed"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type fixture struct {
	svc    *service.Service
	ctx    context.Context
	mail   *bytes.Buffer
	actors map[string]domain.Actor
	acc    map[string]int64 // account code -> id
	now    time.Time
}

var meta = service.Meta{IP: "127.0.0.1", UserAgent: "test"}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mail := &bytes.Buffer{}
	svc := service.New(st, nil, mail)
	svc.BcryptCost = bcrypt.MinCost
	now := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }
	if err := seed.Run(ctx, svc); err != nil {
		t.Fatal(err)
	}
	f := &fixture{svc: svc, ctx: ctx, mail: mail, actors: map[string]domain.Actor{}, acc: map[string]int64{}, now: now}
	users, err := store.ListUsers(ctx, st.DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		f.actors[u.ID] = service.ActorOf(u)
	}
	accs, err := store.ListAccounts(ctx, st.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accs {
		f.acc[a.Code] = a.ID
	}
	return f
}

func (f *fixture) newDraft(t *testing.T, applicant string, items ...domain.ExpenseItem) service.ExpenseDetail {
	t.Helper()
	if len(items) == 0 {
		items = []domain.ExpenseItem{{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 1000, Description: "電車代"}}
	}
	d, err := f.svc.CreateExpense(f.ctx, f.actors[applicant], service.ExpenseInput{Title: "テスト", Items: items}, meta)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *fixture) do(applicantOrActor string, id int64, a domain.Action, comment string) (service.ExpenseDetail, error) {
	return f.svc.Transition(f.ctx, f.actors[applicantOrActor], id, a, comment, meta)
}

func (f *fixture) auditCount(t *testing.T, action string) int {
	t.Helper()
	_, n, err := store.ListAudit(f.ctx, f.svc.Store.DB, store.AuditFilter{Action: action})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHappyPath_SubmitApproveSettle(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	if d.Status != domain.StatusDraft || d.Number == "" {
		t.Fatalf("unexpected draft %+v", d.Expense)
	}

	d, err := f.do("employee1", d.ID, domain.ActionSubmit, "")
	if err != nil || d.Status != domain.StatusSubmitted || d.AppliedAt == nil {
		t.Fatalf("submit: %v %+v", err, d.Expense)
	}
	d, err = f.do("manager1", d.ID, domain.ActionApprove, "問題なし")
	if err != nil || d.Status != domain.StatusApprovedByMgr {
		t.Fatalf("approve: %v", err)
	}
	d, err = f.do("admin1", d.ID, domain.ActionSettle, "精算します")
	if err != nil || d.Status != domain.StatusSettled || d.SettledAt == nil {
		t.Fatalf("settle: %v", err)
	}
	if len(d.History) != 3 || d.History[1].Comment != "問題なし" || d.History[2].ActorID != "admin1" {
		t.Fatalf("history not recorded: %+v", d.History)
	}
	for _, a := range []string{"EXPENSE_SUBMIT", "EXPENSE_APPROVE", "EXPENSE_SETTLE"} {
		if f.auditCount(t, a) == 0 {
			t.Errorf("missing audit %s", a)
		}
	}
}

func TestInvalidTransitionsAreRejected(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")

	// DRAFT cannot be approved or settled.
	if _, err := f.do("manager1", d.ID, domain.ActionApprove, "x"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("approve draft: %v", err)
	}
	if _, err := f.do("admin1", d.ID, domain.ActionSettle, "x"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("settle draft: %v", err)
	}
	if _, err := f.do("employee1", d.ID, domain.ActionSubmit, ""); err != nil {
		t.Fatal(err)
	}
	// SUBMITTED cannot be settled before manager approval.
	if _, err := f.do("admin1", d.ID, domain.ActionSettle, "x"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("settle submitted: %v", err)
	}
	if _, err := f.do("manager1", d.ID, domain.ActionReject, "要修正"); err != nil {
		t.Fatal(err)
	}
	// REJECTED is terminal: never jump to SETTLED.
	for _, tc := range []struct {
		actor  string
		action domain.Action
	}{{"admin1", domain.ActionSettle}, {"manager1", domain.ActionApprove}, {"employee1", domain.ActionSubmit}, {"employee1", domain.ActionWithdraw}} {
		if _, err := f.do(tc.actor, d.ID, tc.action, "x"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Errorf("%s on REJECTED by %s: want invalid transition, got %v", tc.action, tc.actor, err)
		}
	}
	got, _ := f.svc.GetExpense(f.ctx, f.actors["employee1"], d.ID)
	if got.Status != domain.StatusRejected {
		t.Fatalf("status changed unexpectedly: %s", got.Status)
	}
}

func TestWithdrawReturnsToDraft(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	_, _ = f.do("employee1", d.ID, domain.ActionSubmit, "")
	_, _ = f.do("manager1", d.ID, domain.ActionApprove, "ok")
	d, err := f.do("employee1", d.ID, domain.ActionWithdraw, "")
	if err != nil || d.Status != domain.StatusDraft || d.AppliedAt != nil || !d.CanEdit {
		t.Fatalf("withdraw: %v %+v", err, d.Expense)
	}
	// Re-submission goes back to the manager.
	d, err = f.do("employee1", d.ID, domain.ActionSubmit, "")
	if err != nil || d.Status != domain.StatusSubmitted {
		t.Fatalf("resubmit: %v", err)
	}
}

func TestAuthorizationOnTransitions(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	if _, err := f.do("employee2", d.ID, domain.ActionSubmit, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other user submit: %v", err)
	}
	_, _ = f.do("employee1", d.ID, domain.ActionSubmit, "")
	if _, err := f.do("manager2", d.ID, domain.ActionApprove, "ok"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other dept manager approve: %v", err)
	}
	if _, err := f.do("employee1", d.ID, domain.ActionApprove, "ok"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self approve: %v", err)
	}
}

func TestCommentRequired(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	_, _ = f.do("employee1", d.ID, domain.ActionSubmit, "")
	var ve *domain.ValidationError
	if _, err := f.do("manager1", d.ID, domain.ActionReject, "  "); !errors.As(err, &ve) {
		t.Fatalf("reject without comment: %v", err)
	}
	if _, err := f.do("manager1", d.ID, domain.ActionApprove, ""); !errors.As(err, &ve) {
		t.Fatalf("approve without comment: %v", err)
	}
}

func TestSubmitBusinessRules(t *testing.T) {
	f := newFixture(t)
	var ve *domain.ValidationError

	zero := f.newDraft(t, "employee1", domain.ExpenseItem{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 0})
	if _, err := f.do("employee1", zero.ID, domain.ActionSubmit, ""); !errors.As(err, &ve) {
		t.Fatalf("zero amount must not submit: %v", err)
	}

	high := f.newDraft(t, "employee1", domain.ExpenseItem{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 30000, Description: "短い摘要"})
	if _, err := f.do("employee1", high.ID, domain.ActionSubmit, ""); !errors.As(err, &ve) || !strings.Contains(ve.Error(), "description") {
		t.Fatalf("high amount with short description must not submit: %v", err)
	}
	// Fix the description and submit successfully.
	in := service.ExpenseInput{Title: "修正", Items: []domain.ExpenseItem{{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 30000, Description: strings.Repeat("詳", 50)}}}
	if _, err := f.svc.UpdateExpense(f.ctx, f.actors["employee1"], high.ID, in, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.do("employee1", high.ID, domain.ActionSubmit, ""); err != nil {
		t.Fatalf("fixed claim should submit: %v", err)
	}

	over := f.newDraft(t, "employee1", domain.ExpenseItem{UseDate: "2026-10-01", AccountID: f.acc["6130"], Amount: 10001, Description: "会議"})
	if _, err := f.do("employee1", over.ID, domain.ActionSubmit, ""); !errors.As(err, &ve) {
		t.Fatalf("account limit must be enforced: %v", err)
	}
}

func TestFailedTransitionLeavesNoTrace(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1", domain.ExpenseItem{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 0})
	before := f.auditCount(t, "")
	if _, err := f.do("employee1", d.ID, domain.ActionSubmit, ""); err == nil {
		t.Fatal("expected failure")
	}
	if after := f.auditCount(t, ""); after != before {
		t.Fatalf("failed transition must not write audit logs (%d -> %d)", before, after)
	}
	got, _ := f.svc.GetExpense(f.ctx, f.actors["employee1"], d.ID)
	if got.Status != domain.StatusDraft || len(got.History) != 0 {
		t.Fatalf("failed transition must not change state: %+v", got)
	}
}

func TestEditOnlyDraftByApplicant(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	in := service.ExpenseInput{Title: "x", Items: []domain.ExpenseItem{{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 1}}}
	if _, err := f.svc.UpdateExpense(f.ctx, f.actors["manager1"], d.ID, in, meta); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("manager edit: %v", err)
	}
	_, _ = f.do("employee1", d.ID, domain.ActionSubmit, "")
	if _, err := f.svc.UpdateExpense(f.ctx, f.actors["employee1"], d.ID, in, meta); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("edit submitted: %v", err)
	}
}

func TestListScopes(t *testing.T) {
	f := newFixture(t)
	mine, err := f.svc.ListExpenses(f.ctx, f.actors["employee2"], service.ScopeMine, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range mine {
		if e.ApplicantID != "employee2" {
			t.Fatalf("mine leaked %s", e.ApplicantID)
		}
	}
	appr, _ := f.svc.ListExpenses(f.ctx, f.actors["manager1"], service.ScopeApprovals, "")
	for _, e := range appr {
		if e.Status != domain.StatusSubmitted || e.DepartmentCode != "SALES" {
			t.Fatalf("manager approvals leaked %+v", e)
		}
	}
	if len(appr) != 2 {
		t.Fatalf("manager1 should have 2 pending approvals from seed, got %d", len(appr))
	}
	adm, _ := f.svc.ListExpenses(f.ctx, f.actors["admin1"], service.ScopeApprovals, "")
	for _, e := range adm {
		if e.Status != domain.StatusApprovedByMgr {
			t.Fatalf("admin approvals leaked %+v", e)
		}
	}
	if _, err := f.svc.ListExpenses(f.ctx, f.actors["employee1"], service.ScopeAll, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("employee all: %v", err)
	}
	if _, err := f.svc.ListExpenses(f.ctx, f.actors["employee1"], service.ScopeApprovals, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("employee approvals: %v", err)
	}
}

func TestMastersAdminOnlyAndAudited(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.CreateAccount(f.ctx, f.actors["manager1"], service.AccountInput{Code: "9999", Name: "x"}, meta); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("manager create account: %v", err)
	}
	acc, err := f.svc.CreateAccount(f.ctx, f.actors["admin1"], service.AccountInput{Code: "9999", Name: "テスト科目"}, meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateAccount(f.ctx, f.actors["admin1"], service.AccountInput{Code: "9999", Name: "dup"}, meta); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate code: %v", err)
	}
	if _, err := f.svc.UpdateAccount(f.ctx, f.actors["admin1"], acc.ID, service.AccountInput{Code: "9999", Name: "変更後"}, meta); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteAccount(f.ctx, f.actors["admin1"], acc.ID, meta); err != nil {
		t.Fatal(err)
	}
	// Accounts in use cannot be deleted.
	if err := f.svc.DeleteAccount(f.ctx, f.actors["admin1"], f.acc["6110"], meta); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("delete used account: %v", err)
	}
	logs, _, _ := store.ListAudit(f.ctx, f.svc.Store.DB, store.AuditFilter{Action: service.AuditAccountUpdate, Limit: 1})
	if len(logs) != 1 || logs[0].BeforeJSON == nil || !strings.Contains(*logs[0].BeforeJSON, "テスト科目") || !strings.Contains(*logs[0].AfterJSON, "変更後") {
		t.Fatalf("update audit must contain before/after: %+v", logs)
	}
	if f.auditCount(t, service.AuditAccountDelete) != 1 {
		t.Fatal("delete not audited")
	}
}

func TestLoginAudited(t *testing.T) {
	f := newFixture(t)
	if _, _, err := f.svc.Login(f.ctx, "employee1", "wrong", meta); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("bad password: %v", err)
	}
	if _, _, err := f.svc.Login(f.ctx, "nobody", "x", meta); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("unknown user: %v", err)
	}
	token, u, err := f.svc.Login(f.ctx, "employee1", seed.DefaultPassword, meta)
	if err != nil || u.ID != "employee1" {
		t.Fatalf("login: %v", err)
	}
	if got, err := f.svc.Authenticate(f.ctx, token); err != nil || got.ID != "employee1" {
		t.Fatalf("authenticate: %v", err)
	}
	if f.auditCount(t, service.AuditLoginFailure) != 2 || f.auditCount(t, service.AuditLoginSuccess) != 1 {
		t.Fatal("login attempts must be audited")
	}
	if err := f.svc.Logout(f.ctx, token, f.actors["employee1"], meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, token); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("session must be invalid after logout: %v", err)
	}
}

func TestReminderBatch(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.RunReminders(f.ctx, f.actors["manager1"], meta); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("manager batch: %v", err)
	}
	res, err := f.svc.RunReminders(f.ctx, service.SystemActor, meta)
	if err != nil {
		t.Fatal(err)
	}
	// Seed has one stale SUBMITTED (SALES -> manager1) and one stale APPROVED_BY_MGR (-> admin1).
	if res.Targets != 2 || len(res.Reminders) != 2 {
		t.Fatalf("unexpected result %+v", res)
	}
	recipients := map[string]bool{}
	for _, r := range res.Reminders {
		recipients[r.RecipientID] = true
	}
	if !recipients["manager1"] || !recipients["admin1"] {
		t.Fatalf("wrong recipients %v", recipients)
	}
	if !strings.Contains(f.mail.String(), "[DUMMY MAIL] to=manager1") {
		t.Fatalf("dummy mail not written: %s", f.mail.String())
	}
	if f.auditCount(t, service.AuditBatchReminder) != 1 {
		t.Fatal("batch run not audited")
	}
}

func TestExportSettledCSV(t *testing.T) {
	f := newFixture(t)
	var buf bytes.Buffer
	if _, err := f.svc.ExportSettledCSV(f.ctx, f.actors["manager1"], "2026-09-01", "2026-10-31", meta, &buf); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("manager export: %v", err)
	}
	n, err := f.svc.ExportSettledCSV(f.ctx, f.actors["admin1"], "2026-09-01", "2026-10-31", meta, &buf)
	if err != nil {
		t.Fatal(err)
	}
	// Seed has two settled claims with 1 + 2 lines.
	if n != 3 {
		t.Fatalf("expected 3 rows, got %d\n%s", n, buf.String())
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\xEF\xBB\xBF精算日,申請番号") || strings.Contains(out, "SUBMITTED") {
		t.Fatalf("unexpected csv:\n%s", out)
	}
	// A range excluding the settlements yields only the header.
	buf.Reset()
	n, _ = f.svc.ExportSettledCSV(f.ctx, f.actors["admin1"], "2026-01-01", "2026-01-31", meta, &buf)
	if n != 0 {
		t.Fatalf("expected empty export, got %d", n)
	}
	var ve *domain.ValidationError
	if _, err := f.svc.ExportSettledCSV(f.ctx, f.actors["admin1"], "2026-10-31", "2026-09-01", meta, &buf); !errors.As(err, &ve) {
		t.Fatalf("reversed range: %v", err)
	}
}

func TestAuditChainValidAfterOperations(t *testing.T) {
	f := newFixture(t)
	d := f.newDraft(t, "employee1")
	_, _ = f.do("employee1", d.ID, domain.ActionSubmit, "")
	v, err := f.svc.VerifyAudit(f.ctx, f.actors["admin1"])
	if err != nil || !v.Valid || v.Checked == 0 {
		t.Fatalf("audit chain invalid: %+v %v", v, err)
	}
	if _, err := f.svc.VerifyAudit(f.ctx, f.actors["employee1"]); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("employee verify: %v", err)
	}
}

func TestAttachmentAccess(t *testing.T) {
	f := newFixture(t)
	att, err := f.svc.UploadAttachment(f.ctx, f.actors["employee1"], "../../receipt.pdf", "application/pdf", []byte("%PDF-dummy"))
	if err != nil || att.Filename != "receipt.pdf" {
		t.Fatalf("upload: %v %+v", err, att)
	}
	// Another employee cannot attach or read it.
	_, err = f.svc.CreateExpense(f.ctx, f.actors["employee2"], service.ExpenseInput{Items: []domain.ExpenseItem{{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 1, AttachmentID: &att.ID}}}, meta)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("foreign attachment must be rejected: %v", err)
	}
	if _, err := f.svc.GetAttachment(f.ctx, f.actors["employee2"], att.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("foreign read: %v", err)
	}
	d := f.newDraft(t, "employee1", domain.ExpenseItem{UseDate: "2026-10-01", AccountID: f.acc["6110"], Amount: 1, AttachmentID: &att.ID})
	if d.Items[0].AttachmentName == nil || *d.Items[0].AttachmentName != "receipt.pdf" {
		t.Fatalf("attachment not linked: %+v", d.Items[0])
	}
	// The department manager can read it once linked to a visible claim.
	if _, err := f.svc.GetAttachment(f.ctx, f.actors["manager1"], att.ID); err != nil {
		t.Fatalf("manager read: %v", err)
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

// ExpenseInput is the editable content of an expense (header + lines).
type ExpenseInput struct {
	Title string               `json:"title"`
	Items []domain.ExpenseItem `json:"items"`
}

func (in *ExpenseInput) normalize() {
	in.Title = strings.TrimSpace(in.Title)
	if in.Items == nil {
		in.Items = []domain.ExpenseItem{}
	}
	for i := range in.Items {
		in.Items[i].Description = strings.TrimSpace(in.Items[i].Description)
	}
}

// ExpenseDetail is an expense with lines, history and what the viewer may do next.
type ExpenseDetail struct {
	store.Expense
	Items          []store.ExpenseItem  `json:"items"`
	History        []store.HistoryEntry `json:"history"`
	AllowedActions []domain.Action      `json:"allowedActions"`
	CanEdit        bool                 `json:"canEdit"`
}

// Expense list scopes.
const (
	ScopeMine      = "mine"
	ScopeApprovals = "approvals"
	ScopeAll       = "all"
)

// ListExpenses returns expenses visible to the actor for the given scope.
//   - mine:      the actor's own claims
//   - approvals: claims waiting on the actor (managers: SUBMITTED in their
//     department; admins: APPROVED_BY_MGR)
//   - all:       every claim (admins only)
func (s *Service) ListExpenses(ctx context.Context, actor domain.Actor, scope string, status domain.Status) ([]store.Expense, error) {
	f := store.ExpenseFilter{}
	if status != "" {
		if !status.Valid() {
			return nil, badRequest("不正なステータスです")
		}
		f.Statuses = []domain.Status{status}
	}
	switch scope {
	case "", ScopeMine:
		f.ApplicantID = actor.UserID
	case ScopeApprovals:
		f.ExcludeApplicant = actor.UserID
		switch {
		case actor.Role.Can(domain.PermSettle):
			f.Statuses = []domain.Status{domain.StatusApprovedByMgr}
		case actor.Role.Can(domain.PermApproveDept):
			f.DepartmentID = actor.DepartmentID
			f.Statuses = []domain.Status{domain.StatusSubmitted}
		default:
			return nil, domain.ErrForbidden
		}
	case ScopeAll:
		if err := actor.Require(domain.PermViewAllExpenses); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("不正なscopeです")
	}
	return store.ListExpenses(ctx, s.Store.DB, f)
}

func (s *Service) loadDetail(ctx context.Context, q store.DBTX, actor domain.Actor, id int64) (ExpenseDetail, error) {
	e, err := store.GetExpense(ctx, q, id)
	if err != nil {
		return ExpenseDetail{}, err
	}
	if !domain.CanView(actor, e.Ref()) {
		return ExpenseDetail{}, domain.ErrForbidden
	}
	items, err := store.ListExpenseItems(ctx, q, id)
	if err != nil {
		return ExpenseDetail{}, err
	}
	hist, err := store.ListHistory(ctx, q, id)
	if err != nil {
		return ExpenseDetail{}, err
	}
	return ExpenseDetail{
		Expense:        e,
		Items:          items,
		History:        hist,
		AllowedActions: domain.AllowedActions(actor, e.Ref()),
		CanEdit:        domain.CanEdit(actor, e.Ref()),
	}, nil
}

// GetExpense returns an expense if the actor may view it.
func (s *Service) GetExpense(ctx context.Context, actor domain.Actor, id int64) (ExpenseDetail, error) {
	return s.loadDetail(ctx, s.Store.DB, actor, id)
}

// validateAttachments ensures referenced files exist and were uploaded by the actor.
func validateAttachments(ctx context.Context, tx store.DBTX, actor domain.Actor, items []domain.ExpenseItem) error {
	v := &domain.ValidationError{}
	for i, it := range items {
		if it.AttachmentID == nil {
			continue
		}
		att, err := store.GetAttachment(ctx, tx, *it.AttachmentID)
		if errors.Is(err, domain.ErrNotFound) || (err == nil && att.UploadedBy != actor.UserID) {
			v.Add(fmt.Sprintf("items[%d].attachmentId", i), "添付ファイルが存在しません")
			continue
		}
		if err != nil {
			return err
		}
	}
	return v.OrNil()
}

// CreateExpense creates a new draft owned by the actor.
func (s *Service) CreateExpense(ctx context.Context, actor domain.Actor, in ExpenseInput, meta Meta) (ExpenseDetail, error) {
	in.normalize()
	var out ExpenseDetail
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		rules, err := store.AccountRules(ctx, tx)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraft(in.Title, in.Items, rules); err != nil {
			return err
		}
		if err := validateAttachments(ctx, tx, actor, in.Items); err != nil {
			return err
		}
		id, err := store.InsertExpense(ctx, tx, actor.UserID, actor.DepartmentID, in.Title, domain.TotalAmount(in.Items), s.now())
		if err != nil {
			return err
		}
		if err := store.ReplaceExpenseItems(ctx, tx, id, in.Items); err != nil {
			return err
		}
		if out, err = s.loadDetail(ctx, tx, actor, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditExpenseCreate, "expense", itoa(id), nil, snapshot(out))
	})
	return out, err
}

// UpdateExpense replaces the content of a draft. Only the applicant may do so.
func (s *Service) UpdateExpense(ctx context.Context, actor domain.Actor, id int64, in ExpenseInput, meta Meta) (ExpenseDetail, error) {
	in.normalize()
	var out ExpenseDetail
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := s.loadDetail(ctx, tx, actor, id)
		if err != nil {
			return err
		}
		if before.ApplicantID != actor.UserID {
			return domain.ErrForbidden
		}
		if !before.CanEdit {
			return fmt.Errorf("%w: 下書き以外は編集できません", domain.ErrInvalidTransition)
		}
		rules, err := store.AccountRules(ctx, tx)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraft(in.Title, in.Items, rules); err != nil {
			return err
		}
		if err := validateAttachments(ctx, tx, actor, in.Items); err != nil {
			return err
		}
		if err := store.UpdateExpenseContent(ctx, tx, id, in.Title, domain.TotalAmount(in.Items), s.now()); err != nil {
			return err
		}
		if err := store.ReplaceExpenseItems(ctx, tx, id, in.Items); err != nil {
			return err
		}
		if out, err = s.loadDetail(ctx, tx, actor, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditExpenseUpdate, "expense", itoa(id), snapshot(before), snapshot(out))
	})
	return out, err
}

// Transition applies a status-changing action (submit, withdraw, approve, reject, settle).
// Authorization, state-machine checks, business-rule validation, the status
// update, the approval history and the audit record all happen in one transaction.
func (s *Service) Transition(ctx context.Context, actor domain.Actor, id int64, action domain.Action, comment string, meta Meta) (ExpenseDetail, error) {
	if !action.Valid() {
		return ExpenseDetail{}, badRequest("不正な操作です")
	}
	comment = strings.TrimSpace(comment)
	var out ExpenseDetail
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := s.loadDetail(ctx, tx, actor, id)
		if err != nil {
			return err
		}
		next, err := domain.AuthorizeTransition(actor, before.Ref(), action)
		if err != nil {
			return err
		}
		if err := domain.ValidateComment(action, comment); err != nil {
			return err
		}
		if action == domain.ActionSubmit {
			if err := s.validateForSubmit(ctx, tx, before); err != nil {
				return err
			}
		}
		now := s.now()
		if err := store.UpdateExpenseStatus(ctx, tx, id, before.Status, next, now); err != nil {
			return err
		}
		err = store.InsertHistory(ctx, tx, id, store.HistoryEntry{
			Action: action, FromStatus: before.Status, ToStatus: next,
			ActorID: actor.UserID, ActorName: actor.Name, Comment: comment, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if out, err = s.loadDetail(ctx, tx, actor, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditExpenseTransition(action), "expense", itoa(id),
			map[string]any{"status": before.Status},
			map[string]any{"status": next, "comment": comment})
	})
	return out, err
}

func (s *Service) validateForSubmit(ctx context.Context, tx store.DBTX, e ExpenseDetail) error {
	rules, err := store.AccountRules(ctx, tx)
	if err != nil {
		return err
	}
	items := make([]domain.ExpenseItem, len(e.Items))
	for i, it := range e.Items {
		items[i] = domain.ExpenseItem{UseDate: it.UseDate, AccountID: it.AccountID, Amount: it.Amount, Description: it.Description, AttachmentID: it.AttachmentID}
	}
	return domain.ValidateForSubmit(e.Title, items, rules)
}

// snapshot is the audit representation of an expense (no viewer-specific fields).
func snapshot(d ExpenseDetail) map[string]any {
	return map[string]any{
		"number":      d.Number,
		"title":       d.Title,
		"status":      d.Status,
		"totalAmount": d.TotalAmount,
		"items":       d.Items,
	}
}

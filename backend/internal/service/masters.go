package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

var (
	codePattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)
	userIDPattern = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)
)

const minPasswordLength = 8

func validateName(v *domain.ValidationError, field, name string, maxLen int) {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	if n == 0 {
		v.Add(field, "必須項目です")
	} else if n > maxLen {
		v.Add(field, "長すぎます")
	}
}

// ---- Departments ----

// DepartmentInput is the payload to create/update a department.
type DepartmentInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (in DepartmentInput) validate() error {
	v := &domain.ValidationError{}
	if !codePattern.MatchString(in.Code) {
		v.Add("code", "部門コードは英数字・ハイフン・アンダースコア20文字以内で入力してください")
	}
	validateName(v, "name", in.Name, 50)
	return v.OrNil()
}

// ListDepartments is available to every authenticated user (used in forms).
func (s *Service) ListDepartments(ctx context.Context) ([]store.Department, error) {
	return store.ListDepartments(ctx, s.Store.DB)
}

// CreateDepartment adds a department (Admin only).
func (s *Service) CreateDepartment(ctx context.Context, actor domain.Actor, in DepartmentInput, meta Meta) (store.Department, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.Department{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := in.validate(); err != nil {
		return store.Department{}, err
	}
	var out store.Department
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		id, err := store.InsertDepartment(ctx, tx, in.Code, in.Name, s.now())
		if err != nil {
			return conflictOn(err, "部門コードが重複しています")
		}
		if out, err = store.GetDepartment(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditDepartmentCreate, "department", itoa(id), nil, out)
	})
	return out, err
}

// UpdateDepartment modifies a department (Admin only).
func (s *Service) UpdateDepartment(ctx context.Context, actor domain.Actor, id int64, in DepartmentInput, meta Meta) (store.Department, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.Department{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := in.validate(); err != nil {
		return store.Department{}, err
	}
	var out store.Department
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetDepartment(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.UpdateDepartment(ctx, tx, id, in.Code, in.Name, s.now()); err != nil {
			return conflictOn(err, "部門コードが重複しています")
		}
		if out, err = store.GetDepartment(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditDepartmentUpdate, "department", itoa(id), before, out)
	})
	return out, err
}

// DeleteDepartment removes an unused department (Admin only).
func (s *Service) DeleteDepartment(ctx context.Context, actor domain.Actor, id int64, meta Meta) error {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return err
	}
	return s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetDepartment(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.DeleteDepartment(ctx, tx, id); err != nil {
			return conflictOn(err, "所属ユーザーまたは申請が存在するため削除できません")
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditDepartmentDelete, "department", itoa(id), before, nil)
	})
}

// ---- Accounts ----

// AccountInput is the payload to create/update an account.
type AccountInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Active      *bool  `json:"active"`
	LimitAmount *int64 `json:"limitAmount"`
}

func (in AccountInput) toAccount(id int64) (store.Account, error) {
	v := &domain.ValidationError{}
	if !codePattern.MatchString(in.Code) {
		v.Add("code", "科目コードは英数字・ハイフン・アンダースコア20文字以内で入力してください")
	}
	validateName(v, "name", in.Name, 50)
	if in.LimitAmount != nil && *in.LimitAmount <= 0 {
		v.Add("limitAmount", "上限金額は1円以上で入力してください")
	}
	if err := v.OrNil(); err != nil {
		return store.Account{}, err
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	return store.Account{ID: id, Code: in.Code, Name: strings.TrimSpace(in.Name), Active: active, LimitAmount: in.LimitAmount}, nil
}

// ListAccounts lists accounts. Inactive accounts are only shown to master managers.
func (s *Service) ListAccounts(ctx context.Context, actor domain.Actor, includeInactive bool) ([]store.Account, error) {
	if includeInactive {
		if err := actor.Require(domain.PermManageMasters); err != nil {
			return nil, err
		}
	}
	return store.ListAccounts(ctx, s.Store.DB, includeInactive)
}

// CreateAccount adds an account (Admin only).
func (s *Service) CreateAccount(ctx context.Context, actor domain.Actor, in AccountInput, meta Meta) (store.Account, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.Account{}, err
	}
	acc, err := in.toAccount(0)
	if err != nil {
		return store.Account{}, err
	}
	var out store.Account
	err = s.Store.WithTx(ctx, func(tx store.DBTX) error {
		id, err := store.InsertAccount(ctx, tx, acc, s.now())
		if err != nil {
			return conflictOn(err, "科目コードが重複しています")
		}
		if out, err = store.GetAccount(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditAccountCreate, "account", itoa(id), nil, out)
	})
	return out, err
}

// UpdateAccount modifies an account (Admin only).
func (s *Service) UpdateAccount(ctx context.Context, actor domain.Actor, id int64, in AccountInput, meta Meta) (store.Account, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.Account{}, err
	}
	acc, err := in.toAccount(id)
	if err != nil {
		return store.Account{}, err
	}
	var out store.Account
	err = s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.UpdateAccount(ctx, tx, acc, s.now()); err != nil {
			return conflictOn(err, "科目コードが重複しています")
		}
		if out, err = store.GetAccount(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditAccountUpdate, "account", itoa(id), before, out)
	})
	return out, err
}

// DeleteAccount removes an unused account (Admin only).
func (s *Service) DeleteAccount(ctx context.Context, actor domain.Actor, id int64, meta Meta) error {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return err
	}
	return s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.DeleteAccount(ctx, tx, id); err != nil {
			return conflictOn(err, "明細で使用されているため削除できません。無効化してください")
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditAccountDelete, "account", itoa(id), before, nil)
	})
}

// ---- Users ----

// UserInput is the payload to create/update a user. Password is optional on update.
type UserInput struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	DepartmentID int64       `json:"departmentId"`
	Role         domain.Role `json:"role"`
	Password     string      `json:"password"`
}

func (in UserInput) validate(creating bool) error {
	v := &domain.ValidationError{}
	if creating && !userIDPattern.MatchString(in.ID) {
		v.Add("id", "ユーザーIDは英小文字・数字・._-の3〜32文字で入力してください")
	}
	validateName(v, "name", in.Name, 50)
	if in.DepartmentID <= 0 {
		v.Add("departmentId", "所属部門を選択してください")
	}
	if !in.Role.Valid() {
		v.Add("role", "ロールが不正です")
	}
	if (creating || in.Password != "") && utf8.RuneCountInString(in.Password) < minPasswordLength {
		v.Add("password", "パスワードは8文字以上で入力してください")
	}
	return v.OrNil()
}

// ListUsers lists all users (Admin only).
func (s *Service) ListUsers(ctx context.Context, actor domain.Actor) ([]store.User, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return nil, err
	}
	return store.ListUsers(ctx, s.Store.DB)
}

func requireDepartment(ctx context.Context, tx store.DBTX, id int64) error {
	if _, err := store.GetDepartment(ctx, tx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			v := &domain.ValidationError{}
			v.Add("departmentId", "部門が存在しません")
			return v
		}
		return err
	}
	return nil
}

// CreateUser adds a user (Admin only).
func (s *Service) CreateUser(ctx context.Context, actor domain.Actor, in UserInput, meta Meta) (store.User, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.User{}, err
	}
	if err := in.validate(true); err != nil {
		return store.User{}, err
	}
	hash, err := s.HashPassword(in.Password)
	if err != nil {
		return store.User{}, err
	}
	var out store.User
	err = s.Store.WithTx(ctx, func(tx store.DBTX) error {
		if err := requireDepartment(ctx, tx, in.DepartmentID); err != nil {
			return err
		}
		u := store.User{ID: in.ID, Name: strings.TrimSpace(in.Name), DepartmentID: in.DepartmentID, Role: in.Role, PasswordHash: hash}
		if err := store.InsertUser(ctx, tx, u, s.now()); err != nil {
			return conflictOn(err, "ユーザーIDが重複しています")
		}
		if out, err = store.GetUser(ctx, tx, in.ID); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditUserCreate, "user", in.ID, nil, out)
	})
	return out, err
}

// UpdateUser modifies a user (Admin only). An admin cannot change their own role.
func (s *Service) UpdateUser(ctx context.Context, actor domain.Actor, id string, in UserInput, meta Meta) (store.User, error) {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return store.User{}, err
	}
	if err := in.validate(false); err != nil {
		return store.User{}, err
	}
	var out store.User
	err := s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if id == actor.UserID && in.Role != before.Role {
			return badRequest("自分自身のロールは変更できません")
		}
		if err := requireDepartment(ctx, tx, in.DepartmentID); err != nil {
			return err
		}
		u := before
		u.Name, u.DepartmentID, u.Role = strings.TrimSpace(in.Name), in.DepartmentID, in.Role
		if in.Password != "" {
			if u.PasswordHash, err = s.HashPassword(in.Password); err != nil {
				return err
			}
		}
		if err := store.UpdateUser(ctx, tx, u, s.now()); err != nil {
			return err
		}
		if out, err = store.GetUser(ctx, tx, id); err != nil {
			return err
		}
		after := map[string]any{"user": out, "passwordChanged": in.Password != ""}
		return s.audit(ctx, tx, meta, actor.UserID, AuditUserUpdate, "user", id, before, after)
	})
	return out, err
}

// DeleteUser removes a user with no expenses (Admin only). Admins cannot delete themselves.
func (s *Service) DeleteUser(ctx context.Context, actor domain.Actor, id string, meta Meta) error {
	if err := actor.Require(domain.PermManageMasters); err != nil {
		return err
	}
	if id == actor.UserID {
		return badRequest("自分自身は削除できません")
	}
	return s.Store.WithTx(ctx, func(tx store.DBTX) error {
		before, err := store.GetUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.DeleteUser(ctx, tx, id); err != nil {
			return conflictOn(err, "申請データが存在するため削除できません")
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditUserDelete, "user", id, before, nil)
	})
}

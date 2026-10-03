// Package seed loads initial master data, test users and sample expenses.
// Everything goes through the service layer so the audit log and approval
// history are consistent with real usage.
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

// DefaultPassword is the password of every seeded test user.
const DefaultPassword = "Passw0rd!"

// TestUser describes a seeded login account.
type TestUser struct {
	ID         string
	Name       string
	Department string
	Role       domain.Role
}

// TestUsers are the seeded login accounts (documented in README).
var TestUsers = []TestUser{
	{"employee1", "山田 太郎", "SALES", domain.RoleEmployee},
	{"employee2", "佐藤 花子", "DEV", domain.RoleEmployee},
	{"manager1", "鈴木 一郎", "SALES", domain.RoleManager},
	{"manager2", "高橋 次郎", "DEV", domain.RoleManager},
	{"admin1", "田中 経理", "ACCT", domain.RoleAdmin},
}

func ptr(v int64) *int64 { return &v }
func bptr(v bool) *bool  { return &v }

// IfEmpty seeds the database only when it has no users yet.
func IfEmpty(ctx context.Context, svc *service.Service) (bool, error) {
	n, err := store.CountUsers(ctx, svc.Store.DB)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	return true, Run(ctx, svc)
}

// Run inserts the seed data. It temporarily overrides svc.Now to backdate
// sample expenses (so the reminder batch has something to find) and restores it.
func Run(ctx context.Context, svc *service.Service) error {
	realNow := svc.Now
	defer func() { svc.Now = realNow }()
	base := realNow()
	at := func(daysAgo int) {
		svc.Now = func() time.Time { return base.Add(-time.Duration(daysAgo) * 24 * time.Hour) }
	}
	at(30)

	sys := service.SystemActor
	meta := service.Meta{IP: "127.0.0.1", UserAgent: "seed"}

	depts := map[string]int64{}
	for _, d := range []service.DepartmentInput{{Code: "SALES", Name: "営業部"}, {Code: "DEV", Name: "開発部"}, {Code: "ACCT", Name: "経理部"}} {
		out, err := svc.CreateDepartment(ctx, sys, d, meta)
		if err != nil {
			return fmt.Errorf("seed department %s: %w", d.Code, err)
		}
		depts[d.Code] = out.ID
	}

	accs := map[string]int64{}
	for _, a := range []service.AccountInput{
		{Code: "6110", Name: "旅費交通費"},
		{Code: "6120", Name: "交際費", LimitAmount: ptr(50000)},
		{Code: "6130", Name: "会議費", LimitAmount: ptr(10000)},
		{Code: "6140", Name: "消耗品費", LimitAmount: ptr(100000)},
		{Code: "6150", Name: "通信費"},
		{Code: "6190", Name: "雑費（旧）", Active: bptr(false)},
	} {
		out, err := svc.CreateAccount(ctx, sys, a, meta)
		if err != nil {
			return fmt.Errorf("seed account %s: %w", a.Code, err)
		}
		accs[a.Code] = out.ID
	}

	actors := map[string]domain.Actor{}
	for _, u := range TestUsers {
		out, err := svc.CreateUser(ctx, sys, service.UserInput{ID: u.ID, Name: u.Name, DepartmentID: depts[u.Department], Role: u.Role, Password: DefaultPassword}, meta)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", u.ID, err)
		}
		actors[u.ID] = service.ActorOf(out)
	}

	day := func(daysAgo int) string { return base.AddDate(0, 0, -daysAgo).Format(domain.DateLayout) }
	longDesc := "大阪支社での顧客向け製品説明会に参加するための往復新幹線代（東京⇔新大阪）。顧客訪問3件、提案資料の説明および契約条件の調整を実施。"

	type step struct {
		daysAgo int
		actor   string
		action  domain.Action
		comment string
	}
	samples := []struct {
		applicant string
		created   int
		in        service.ExpenseInput
		steps     []step
	}{
		{ // DRAFT
			"employee1", 2,
			service.ExpenseInput{Title: "10月 客先訪問交通費", Items: []domain.ExpenseItem{
				{UseDate: day(2), AccountID: accs["6110"], Amount: 1340, Description: "客先訪問（新宿⇔品川）"},
			}},
			nil,
		},
		{ // SUBMITTED, stale (reminder target for manager1)
			"employee1", 12,
			service.ExpenseInput{Title: "会議用備品購入", Items: []domain.ExpenseItem{
				{UseDate: day(12), AccountID: accs["6140"], Amount: 8800, Description: "会議室用HDMIケーブル・延長コード"},
				{UseDate: day(12), AccountID: accs["6130"], Amount: 3200, Description: "定例会議の茶菓子"},
			}},
			[]step{{10, "employee1", domain.ActionSubmit, ""}},
		},
		{ // SUBMITTED, recent, high amount
			"employee1", 2,
			service.ExpenseInput{Title: "大阪出張", Items: []domain.ExpenseItem{
				{UseDate: day(3), AccountID: accs["6110"], Amount: 29720, Description: longDesc},
				{UseDate: day(3), AccountID: accs["6120"], Amount: 32000, Description: longDesc + "（会食）"},
			}},
			[]step{{1, "employee1", domain.ActionSubmit, ""}},
		},
		{ // APPROVED_BY_MGR, stale (reminder target for admin1)
			"employee2", 15,
			service.ExpenseInput{Title: "開発用書籍・通信費", Items: []domain.ExpenseItem{
				{UseDate: day(15), AccountID: accs["6140"], Amount: 4620, Description: "技術書（Go言語）"},
				{UseDate: day(14), AccountID: accs["6150"], Amount: 2980, Description: "モバイルWi-Fi 9月分"},
			}},
			[]step{{13, "employee2", domain.ActionSubmit, ""}, {9, "manager2", domain.ActionApprove, "業務上必要と認めます"}},
		},
		{ // SETTLED (recent)
			"employee2", 8,
			service.ExpenseInput{Title: "勉強会 会議費", Items: []domain.ExpenseItem{
				{UseDate: day(8), AccountID: accs["6130"], Amount: 6500, Description: "社内勉強会の会場費"},
			}},
			[]step{{7, "employee2", domain.ActionSubmit, ""}, {6, "manager2", domain.ActionApprove, "承認します"}, {3, "admin1", domain.ActionSettle, "精算確定"}},
		},
		{ // SETTLED (older)
			"employee1", 25,
			service.ExpenseInput{Title: "9月 交通費", Items: []domain.ExpenseItem{
				{UseDate: day(25), AccountID: accs["6110"], Amount: 2140, Description: "客先訪問（渋谷⇔横浜）"},
				{UseDate: day(24), AccountID: accs["6110"], Amount: 980, Description: "客先訪問（新宿⇔池袋）"},
			}},
			[]step{{24, "employee1", domain.ActionSubmit, ""}, {23, "manager1", domain.ActionApprove, "OK"}, {20, "admin1", domain.ActionSettle, "振込済み"}},
		},
		{ // REJECTED
			"employee1", 6,
			service.ExpenseInput{Title: "懇親会費", Items: []domain.ExpenseItem{
				{UseDate: day(6), AccountID: accs["6120"], Amount: 12000, Description: "部内懇親会"},
			}},
			[]step{{5, "employee1", domain.ActionSubmit, ""}, {4, "manager1", domain.ActionReject, "部内懇親会は交際費対象外です。福利厚生費で再申請してください"}},
		},
	}

	for i, smp := range samples {
		at(smp.created)
		exp, err := svc.CreateExpense(ctx, actors[smp.applicant], smp.in, meta)
		if err != nil {
			return fmt.Errorf("seed expense %d: %w", i, err)
		}
		for _, st := range smp.steps {
			at(st.daysAgo)
			if _, err := svc.Transition(ctx, actors[st.actor], exp.ID, st.action, st.comment, meta); err != nil {
				return fmt.Errorf("seed expense %d %s: %w", i, st.action, err)
			}
		}
	}
	return nil
}

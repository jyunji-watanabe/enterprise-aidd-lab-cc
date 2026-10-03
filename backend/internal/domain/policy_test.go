package domain

import (
	"errors"
	"reflect"
	"testing"
)

const (
	deptSales int64 = 1
	deptDev   int64 = 2
)

var (
	employee     = Actor{UserID: "emp", Role: RoleEmployee, DepartmentID: deptSales}
	otherEmp     = Actor{UserID: "emp2", Role: RoleEmployee, DepartmentID: deptSales}
	salesManager = Actor{UserID: "mgr", Role: RoleManager, DepartmentID: deptSales}
	devManager   = Actor{UserID: "mgr2", Role: RoleManager, DepartmentID: deptDev}
	admin        = Actor{UserID: "adm", Role: RoleAdmin, DepartmentID: 3}
)

func ref(status Status) ExpenseRef {
	return ExpenseRef{ApplicantID: employee.UserID, DepartmentID: deptSales, Status: status}
}

func TestRolePermissions(t *testing.T) {
	adminOnly := []Permission{PermManageMasters, PermViewAuditLog, PermExportReports, PermRunBatch, PermSettle, PermViewAllExpenses}
	for _, p := range adminOnly {
		if RoleEmployee.Can(p) || RoleManager.Can(p) {
			t.Errorf("%s must be admin only", p)
		}
		if !RoleAdmin.Can(p) {
			t.Errorf("admin must have %s", p)
		}
	}
	if !RoleManager.Can(PermApproveDept) || RoleEmployee.Can(PermApproveDept) {
		t.Error("only managers approve department expenses")
	}
	if err := employee.Require(PermManageMasters); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if Role("Root").Valid() {
		t.Error("unknown role must be invalid")
	}
}

func TestCanView(t *testing.T) {
	e := ref(StatusSubmitted)
	cases := map[string]struct {
		actor Actor
		want  bool
	}{
		"applicant":          {employee, true},
		"same-dept employee": {otherEmp, false},
		"dept manager":       {salesManager, true},
		"other-dept manager": {devManager, false},
		"admin sees all":     {admin, true},
	}
	for name, c := range cases {
		if got := CanView(c.actor, e); got != c.want {
			t.Errorf("%s: CanView = %v, want %v", name, got, c.want)
		}
	}
}

func TestCanEdit(t *testing.T) {
	if !CanEdit(employee, ref(StatusDraft)) {
		t.Error("applicant can edit draft")
	}
	for _, s := range []Status{StatusSubmitted, StatusApprovedByMgr, StatusSettled, StatusRejected} {
		if CanEdit(employee, ref(s)) {
			t.Errorf("applicant must not edit %s", s)
		}
	}
	if CanEdit(salesManager, ref(StatusDraft)) || CanEdit(admin, ref(StatusDraft)) {
		t.Error("only the applicant can edit")
	}
}

func TestAuthorizeTransition(t *testing.T) {
	cases := []struct {
		name   string
		actor  Actor
		status Status
		action Action
		want   Status
		err    error
	}{
		{"applicant submits draft", employee, StatusDraft, ActionSubmit, StatusSubmitted, nil},
		{"other employee cannot submit", otherEmp, StatusDraft, ActionSubmit, "", ErrForbidden},
		{"applicant withdraws submitted", employee, StatusSubmitted, ActionWithdraw, StatusDraft, nil},
		{"applicant withdraws approved", employee, StatusApprovedByMgr, ActionWithdraw, StatusDraft, nil},
		{"manager cannot withdraw others", salesManager, StatusSubmitted, ActionWithdraw, "", ErrForbidden},
		{"cannot withdraw settled", employee, StatusSettled, ActionWithdraw, "", ErrInvalidTransition},
		{"dept manager approves", salesManager, StatusSubmitted, ActionApprove, StatusApprovedByMgr, nil},
		{"dept manager rejects", salesManager, StatusSubmitted, ActionReject, StatusRejected, nil},
		{"other dept manager cannot approve", devManager, StatusSubmitted, ActionApprove, "", ErrForbidden},
		{"employee cannot approve", employee, StatusSubmitted, ActionApprove, "", ErrForbidden},
		{"admin cannot do manager approval", admin, StatusSubmitted, ActionApprove, "", ErrForbidden},
		{"admin cannot reject before manager", admin, StatusSubmitted, ActionReject, "", ErrForbidden},
		{"manager cannot approve twice", salesManager, StatusApprovedByMgr, ActionApprove, "", ErrInvalidTransition},
		{"admin settles", admin, StatusApprovedByMgr, ActionSettle, StatusSettled, nil},
		{"admin rejects approved", admin, StatusApprovedByMgr, ActionReject, StatusRejected, nil},
		{"manager cannot reject after own approval", salesManager, StatusApprovedByMgr, ActionReject, "", ErrForbidden},
		{"manager cannot settle", salesManager, StatusApprovedByMgr, ActionSettle, "", ErrForbidden},
		{"employee cannot settle", employee, StatusApprovedByMgr, ActionSettle, "", ErrForbidden},
		{"admin cannot settle rejected", admin, StatusRejected, ActionSettle, "", ErrInvalidTransition},
		{"admin cannot settle submitted", admin, StatusSubmitted, ActionSettle, "", ErrInvalidTransition},
		{"admin cannot settle draft", admin, StatusDraft, ActionSettle, "", ErrInvalidTransition},
		{"cannot resubmit rejected", employee, StatusRejected, ActionSubmit, "", ErrInvalidTransition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AuthorizeTransition(c.actor, ref(c.status), c.action)
			if c.err != nil {
				if !errors.Is(err, c.err) {
					t.Fatalf("want error %v, got %v (status %s)", c.err, err, got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("want %s, got %s err=%v", c.want, got, err)
			}
		})
	}
}

func TestSelfApprovalIsForbidden(t *testing.T) {
	ownByManager := ExpenseRef{ApplicantID: salesManager.UserID, DepartmentID: deptSales, Status: StatusSubmitted}
	if _, err := AuthorizeTransition(salesManager, ownByManager, ActionApprove); !errors.Is(err, ErrForbidden) {
		t.Fatalf("manager must not approve own claim: %v", err)
	}
	ownByAdmin := ExpenseRef{ApplicantID: admin.UserID, DepartmentID: admin.DepartmentID, Status: StatusApprovedByMgr}
	if _, err := AuthorizeTransition(admin, ownByAdmin, ActionSettle); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin must not settle own claim: %v", err)
	}
}

func TestAllowedActions(t *testing.T) {
	cases := []struct {
		actor  Actor
		status Status
		want   []Action
	}{
		{employee, StatusDraft, []Action{ActionSubmit}},
		{employee, StatusSubmitted, []Action{ActionWithdraw}},
		{salesManager, StatusSubmitted, []Action{ActionApprove, ActionReject}},
		{admin, StatusApprovedByMgr, []Action{ActionReject, ActionSettle}},
		{admin, StatusSettled, []Action{}},
		{devManager, StatusSubmitted, []Action{}},
	}
	for _, c := range cases {
		got := AllowedActions(c.actor, ref(c.status))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s on %s: got %v want %v", c.actor.UserID, c.status, got, c.want)
		}
	}
}

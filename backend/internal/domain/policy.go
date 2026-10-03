package domain

// ExpenseRef is the minimum information about an expense needed for
// authorization decisions.
type ExpenseRef struct {
	ApplicantID  string
	DepartmentID int64
	Status       Status
}

func (e ExpenseRef) isApplicant(a Actor) bool { return e.ApplicantID == a.UserID }

func (e ExpenseRef) isDeptManager(a Actor) bool {
	return a.Role.Can(PermApproveDept) && a.DepartmentID == e.DepartmentID
}

// CanView reports whether the actor may see the expense: the applicant,
// a manager of the expense's department, or anyone who may view all expenses.
func CanView(a Actor, e ExpenseRef) bool {
	return e.isApplicant(a) || e.isDeptManager(a) || a.Role.Can(PermViewAllExpenses)
}

// CanEdit reports whether the actor may modify the expense contents.
// Only the applicant may edit, and only while it is a draft.
func CanEdit(a Actor, e ExpenseRef) bool {
	return e.isApplicant(a) && e.Status == StatusDraft
}

// AuthorizeTransition checks that the actor may perform the action on the
// expense and that the state machine allows it. It returns the next status.
//
// Error precedence: ErrForbidden when the actor's role can never perform the
// action, ErrInvalidTransition when the current status does not allow it, and
// ErrForbidden when the actor is not the right person for this expense
// (e.g. a manager of another department or approving their own claim).
func AuthorizeTransition(a Actor, e ExpenseRef, action Action) (Status, error) {
	if !roleMayPerform(a.Role, action) {
		return "", ErrForbidden
	}
	next, err := NextStatus(e.Status, action)
	if err != nil {
		return "", err
	}
	if !actorMayPerform(a, e, action) {
		return "", ErrForbidden
	}
	return next, nil
}

// AllowedActions lists the actions the actor may currently perform on the expense.
func AllowedActions(a Actor, e ExpenseRef) []Action {
	out := []Action{}
	for _, act := range AllActions {
		if _, err := AuthorizeTransition(a, e, act); err == nil {
			out = append(out, act)
		}
	}
	return out
}

func roleMayPerform(r Role, action Action) bool {
	switch action {
	case ActionSubmit, ActionWithdraw:
		return r.Valid()
	case ActionApprove:
		return r.Can(PermApproveDept)
	case ActionReject:
		return r.Can(PermApproveDept) || r.Can(PermSettle)
	case ActionSettle:
		return r.Can(PermSettle)
	}
	return false
}

func actorMayPerform(a Actor, e ExpenseRef, action Action) bool {
	switch action {
	case ActionSubmit, ActionWithdraw:
		return e.isApplicant(a)
	case ActionApprove:
		return e.isDeptManager(a) && !e.isApplicant(a)
	case ActionSettle:
		return a.Role.Can(PermSettle) && !e.isApplicant(a)
	case ActionReject:
		switch e.Status {
		case StatusSubmitted:
			return e.isDeptManager(a) && !e.isApplicant(a)
		case StatusApprovedByMgr:
			return a.Role.Can(PermSettle) && !e.isApplicant(a)
		}
	}
	return false
}

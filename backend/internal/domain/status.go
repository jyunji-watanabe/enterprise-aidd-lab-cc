package domain

// Status is the lifecycle status of an expense claim.
type Status string

// Expense statuses.
const (
	StatusDraft         Status = "DRAFT"
	StatusSubmitted     Status = "SUBMITTED"
	StatusApprovedByMgr Status = "APPROVED_BY_MGR"
	StatusSettled       Status = "SETTLED"
	StatusRejected      Status = "REJECTED"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusSubmitted, StatusApprovedByMgr, StatusSettled, StatusRejected:
		return true
	}
	return false
}

// Action is an operation that changes the status of an expense.
type Action string

// Status-changing actions.
const (
	ActionSubmit   Action = "submit"
	ActionWithdraw Action = "withdraw"
	ActionApprove  Action = "approve"
	ActionReject   Action = "reject"
	ActionSettle   Action = "settle"
)

// AllActions lists every status-changing action in display order.
var AllActions = []Action{ActionSubmit, ActionWithdraw, ActionApprove, ActionReject, ActionSettle}

// Valid reports whether a is a known action.
func (a Action) Valid() bool {
	for _, x := range AllActions {
		if x == a {
			return true
		}
	}
	return false
}

// RequiresComment reports whether the action must be accompanied by a comment
// (approval and rejection require one).
func (a Action) RequiresComment() bool {
	return a == ActionApprove || a == ActionReject || a == ActionSettle
}

type transitionKey struct {
	from   Status
	action Action
}

// transitions is the complete state machine. Any (status, action) pair not
// listed here is rejected.
var transitions = map[transitionKey]Status{
	{StatusDraft, ActionSubmit}:           StatusSubmitted,
	{StatusSubmitted, ActionApprove}:      StatusApprovedByMgr,
	{StatusSubmitted, ActionReject}:       StatusRejected,
	{StatusApprovedByMgr, ActionSettle}:   StatusSettled,
	{StatusApprovedByMgr, ActionReject}:   StatusRejected,
	{StatusSubmitted, ActionWithdraw}:     StatusDraft,
	{StatusApprovedByMgr, ActionWithdraw}: StatusDraft,
}

// NextStatus returns the status reached by applying action to from, or
// ErrInvalidTransition if the state machine does not allow it.
func NextStatus(from Status, action Action) (Status, error) {
	to, ok := transitions[transitionKey{from, action}]
	if !ok {
		return "", ErrInvalidTransition
	}
	return to, nil
}

// IsAwaitingApproval reports whether an expense in status s is waiting on an approver.
func (s Status) IsAwaitingApproval() bool {
	return s == StatusSubmitted || s == StatusApprovedByMgr
}

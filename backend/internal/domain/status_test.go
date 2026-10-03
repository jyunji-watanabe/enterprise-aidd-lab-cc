package domain

import (
	"errors"
	"testing"
)

var allStatuses = []Status{StatusDraft, StatusSubmitted, StatusApprovedByMgr, StatusSettled, StatusRejected}

func TestNextStatus_AllowedTransitions(t *testing.T) {
	cases := []struct {
		from   Status
		action Action
		want   Status
	}{
		{StatusDraft, ActionSubmit, StatusSubmitted},
		{StatusSubmitted, ActionApprove, StatusApprovedByMgr},
		{StatusSubmitted, ActionReject, StatusRejected},
		{StatusApprovedByMgr, ActionSettle, StatusSettled},
		{StatusApprovedByMgr, ActionReject, StatusRejected},
		{StatusSubmitted, ActionWithdraw, StatusDraft},
		{StatusApprovedByMgr, ActionWithdraw, StatusDraft},
	}
	for _, c := range cases {
		got, err := NextStatus(c.from, c.action)
		if err != nil {
			t.Errorf("%s --%s--> expected %s, got error %v", c.from, c.action, c.want, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s --%s--> got %s, want %s", c.from, c.action, got, c.want)
		}
	}
}

// TestNextStatus_RejectsEverythingElse enumerates the full (status x action)
// matrix and verifies that exactly the 7 specified transitions are allowed.
func TestNextStatus_RejectsEverythingElse(t *testing.T) {
	allowed := 0
	for _, s := range allStatuses {
		for _, a := range AllActions {
			_, err := NextStatus(s, a)
			if err == nil {
				allowed++
				continue
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s/%s: unexpected error %v", s, a, err)
			}
		}
	}
	if allowed != 7 {
		t.Fatalf("expected exactly 7 allowed transitions, got %d", allowed)
	}
}

func TestNextStatus_TerminalStates(t *testing.T) {
	for _, s := range []Status{StatusSettled, StatusRejected} {
		for _, a := range AllActions {
			if _, err := NextStatus(s, a); !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("terminal status %s must reject %s", s, a)
			}
		}
	}
	// Specifically: REJECTED must never jump to SETTLED, DRAFT must not skip approval.
	if _, err := NextStatus(StatusRejected, ActionSettle); err == nil {
		t.Fatal("REJECTED -> SETTLED must be rejected")
	}
	if _, err := NextStatus(StatusDraft, ActionSettle); err == nil {
		t.Fatal("DRAFT -> SETTLED must be rejected")
	}
	if _, err := NextStatus(StatusSubmitted, ActionSettle); err == nil {
		t.Fatal("SUBMITTED -> SETTLED must be rejected (manager approval required first)")
	}
}

func TestActionRequiresComment(t *testing.T) {
	want := map[Action]bool{ActionSubmit: false, ActionWithdraw: false, ActionApprove: true, ActionReject: true, ActionSettle: true}
	for a, w := range want {
		if a.RequiresComment() != w {
			t.Errorf("%s RequiresComment = %v, want %v", a, !w, w)
		}
	}
}

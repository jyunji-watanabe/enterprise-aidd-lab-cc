package domain

import (
	"errors"
	"strings"
	"testing"
)

func ptr(v int64) *int64 { return &v }

var testAccounts = map[int64]AccountRule{
	1: {ID: 1, Name: "旅費交通費", Active: true},
	2: {ID: 2, Name: "会議費", Active: true, LimitAmount: ptr(10000)},
	3: {ID: 3, Name: "旧科目", Active: false},
}

func item(amount int64, desc string) ExpenseItem {
	return ExpenseItem{UseDate: "2026-09-01", AccountID: 1, Amount: amount, Description: desc}
}

func fields(err error) []string {
	var ve *ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	out := []string{}
	for _, fe := range ve.Errors {
		out = append(out, fe.Field)
	}
	return out
}

func hasField(err error, field string) bool {
	for _, f := range fields(err) {
		if f == field {
			return true
		}
	}
	return false
}

func TestValidateForSubmit_OK(t *testing.T) {
	items := []ExpenseItem{item(1200, "電車代"), {UseDate: "2026-09-02", AccountID: 2, Amount: 10000, Description: "会議"}}
	if err := ValidateForSubmit("出張", items, testAccounts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateForSubmit_TotalMustBePositive(t *testing.T) {
	if err := ValidateForSubmit("", nil, testAccounts); !hasField(err, "totalAmount") || !hasField(err, "items") {
		t.Fatalf("empty claim must be rejected, got %v", err)
	}
	if err := ValidateForSubmit("", []ExpenseItem{item(0, "")}, testAccounts); !hasField(err, "totalAmount") {
		t.Fatalf("zero total must be rejected, got %v", err)
	}
}

func TestValidateForSubmit_HighAmountDescription(t *testing.T) {
	desc49 := strings.Repeat("あ", 49)
	desc50 := strings.Repeat("あ", 50)

	if err := ValidateForSubmit("", []ExpenseItem{item(29999, "短い")}, testAccounts); err != nil {
		t.Fatalf("29,999 yen needs no long description: %v", err)
	}
	if err := ValidateForSubmit("", []ExpenseItem{item(30000, desc49)}, testAccounts); !hasField(err, "items[0].description") {
		t.Fatalf("30,000 yen with 49 chars must fail, got %v", err)
	}
	if err := ValidateForSubmit("", []ExpenseItem{item(30000, desc50)}, testAccounts); err != nil {
		t.Fatalf("30,000 yen with 50 chars must pass: %v", err)
	}
	// Whitespace padding does not count toward the requirement.
	padded := strings.Repeat("あ", 10) + strings.Repeat(" ", 45)
	if err := ValidateForSubmit("", []ExpenseItem{item(50000, padded)}, testAccounts); !hasField(err, "items[0].description") {
		t.Fatalf("whitespace must not satisfy description length, got %v", err)
	}
}

func TestValidateForSubmit_AccountRules(t *testing.T) {
	over := ExpenseItem{UseDate: "2026-09-01", AccountID: 2, Amount: 10001, Description: "会議"}
	if err := ValidateForSubmit("", []ExpenseItem{over}, testAccounts); !hasField(err, "items[0].amount") {
		t.Fatalf("account limit must be enforced, got %v", err)
	}
	inactive := ExpenseItem{UseDate: "2026-09-01", AccountID: 3, Amount: 100}
	if err := ValidateForSubmit("", []ExpenseItem{inactive}, testAccounts); !hasField(err, "items[0].accountId") {
		t.Fatalf("inactive account must be rejected, got %v", err)
	}
}

func TestValidateDraft(t *testing.T) {
	// Drafts may be incomplete (zero amount, empty description)...
	if err := ValidateDraft("", []ExpenseItem{item(0, "")}, testAccounts); err != nil {
		t.Fatalf("incomplete draft should be savable: %v", err)
	}
	// ...but must be structurally valid.
	bad := []ExpenseItem{{UseDate: "2026/09/01", AccountID: 99, Amount: -1}}
	err := ValidateDraft("", bad, testAccounts)
	for _, f := range []string{"items[0].useDate", "items[0].accountId", "items[0].amount"} {
		if !hasField(err, f) {
			t.Errorf("expected error on %s, got %v", f, err)
		}
	}
}

func TestValidateComment(t *testing.T) {
	for _, a := range []Action{ActionApprove, ActionReject, ActionSettle} {
		if err := ValidateComment(a, "   "); !hasField(err, "comment") {
			t.Errorf("%s requires comment", a)
		}
		if err := ValidateComment(a, "OK"); err != nil {
			t.Errorf("%s with comment: %v", a, err)
		}
	}
	if err := ValidateComment(ActionSubmit, ""); err != nil {
		t.Errorf("submit does not require comment: %v", err)
	}
}

func TestTotalAmount(t *testing.T) {
	if got := TotalAmount([]ExpenseItem{item(100, ""), item(250, "")}); got != 350 {
		t.Fatalf("got %d", got)
	}
}

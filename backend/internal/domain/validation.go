package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Business rule constants.
const (
	// HighAmountThreshold is the per-line amount (JPY, inclusive) from which a
	// detailed description is mandatory.
	HighAmountThreshold int64 = 30000
	// MinDescriptionForHighAmount is the minimum description length (characters)
	// for lines at or above HighAmountThreshold.
	MinDescriptionForHighAmount = 50
	// MaxItems limits the number of lines on one expense.
	MaxItems = 100
	// MaxDescriptionLength limits the description length (characters).
	MaxDescriptionLength = 500
	// MaxTitleLength limits the title length (characters).
	MaxTitleLength = 100
	// MaxCommentLength limits approval comments (characters).
	MaxCommentLength = 1000
	// DateLayout is the format of dates exchanged with clients.
	DateLayout = "2006-01-02"
)

// ExpenseItem is one line of an expense claim.
type ExpenseItem struct {
	UseDate      string `json:"useDate"`
	AccountID    int64  `json:"accountId"`
	Amount       int64  `json:"amount"`
	Description  string `json:"description"`
	AttachmentID *int64 `json:"attachmentId"`
}

// AccountRule is the subset of account master data relevant to validation.
type AccountRule struct {
	ID          int64
	Name        string
	Active      bool
	LimitAmount *int64
}

// TotalAmount sums the amounts of the items.
func TotalAmount(items []ExpenseItem) int64 {
	var total int64
	for _, it := range items {
		total += it.Amount
	}
	return total
}

// ValidateDraft checks structural constraints that must hold even for drafts:
// well-formed dates, existing accounts, non-negative amounts and length limits.
func ValidateDraft(title string, items []ExpenseItem, accounts map[int64]AccountRule) error {
	v := &ValidationError{}
	if utf8.RuneCountInString(title) > MaxTitleLength {
		v.Add("title", fmt.Sprintf("件名は%d文字以内で入力してください", MaxTitleLength))
	}
	if len(items) > MaxItems {
		v.Add("items", fmt.Sprintf("明細は%d行以内にしてください", MaxItems))
	}
	for i, it := range items {
		f := func(name string) string { return fmt.Sprintf("items[%d].%s", i, name) }
		if _, err := time.Parse(DateLayout, it.UseDate); err != nil {
			v.Add(f("useDate"), "利用日はYYYY-MM-DD形式で入力してください")
		}
		if _, ok := accounts[it.AccountID]; !ok {
			v.Add(f("accountId"), "勘定科目が存在しません")
		}
		if it.Amount < 0 {
			v.Add(f("amount"), "金額は0円以上で入力してください")
		}
		if utf8.RuneCountInString(it.Description) > MaxDescriptionLength {
			v.Add(f("description"), fmt.Sprintf("摘要は%d文字以内で入力してください", MaxDescriptionLength))
		}
	}
	return v.OrNil()
}

// ValidateForSubmit checks every business rule required to submit an expense.
// It includes the draft checks.
func ValidateForSubmit(title string, items []ExpenseItem, accounts map[int64]AccountRule) error {
	if err := ValidateDraft(title, items, accounts); err != nil {
		return err
	}
	v := &ValidationError{}
	if len(items) == 0 {
		v.Add("items", "明細を1行以上入力してください")
	}
	if TotalAmount(items) <= 0 {
		v.Add("totalAmount", "申請金額が0円以下のため提出できません")
	}
	for i, it := range items {
		f := func(name string) string { return fmt.Sprintf("items[%d].%s", i, name) }
		if it.Amount <= 0 {
			v.Add(f("amount"), "金額は1円以上で入力してください")
		}
		acc := accounts[it.AccountID]
		if !acc.Active {
			v.Add(f("accountId"), fmt.Sprintf("勘定科目「%s」は無効です", acc.Name))
		}
		if acc.LimitAmount != nil && it.Amount > *acc.LimitAmount {
			v.Add(f("amount"), fmt.Sprintf("勘定科目「%s」の上限金額(%d円)を超えています", acc.Name, *acc.LimitAmount))
		}
		if it.Amount >= HighAmountThreshold &&
			utf8.RuneCountInString(strings.TrimSpace(it.Description)) < MinDescriptionForHighAmount {
			v.Add(f("description"), fmt.Sprintf("%d円以上の明細は摘要を%d文字以上入力してください", HighAmountThreshold, MinDescriptionForHighAmount))
		}
	}
	return v.OrNil()
}

// ValidateComment enforces the mandatory comment on approval/rejection/settlement.
func ValidateComment(action Action, comment string) error {
	v := &ValidationError{}
	trimmed := strings.TrimSpace(comment)
	if action.RequiresComment() && trimmed == "" {
		v.Add("comment", "承認・差戻し時はコメントの入力が必須です")
	}
	if utf8.RuneCountInString(trimmed) > MaxCommentLength {
		v.Add("comment", fmt.Sprintf("コメントは%d文字以内で入力してください", MaxCommentLength))
	}
	return v.OrNil()
}

// Package service implements the application use cases. Every state change
// runs in a single database transaction together with its audit record.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	// Embed the timezone database so Asia/Tokyo works in minimal containers.
	_ "time/tzdata"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// Audit action names.
const (
	AuditLoginSuccess     = "LOGIN_SUCCESS"
	AuditLoginFailure     = "LOGIN_FAILURE"
	AuditLogout           = "LOGOUT"
	AuditExpenseCreate    = "EXPENSE_CREATE"
	AuditExpenseUpdate    = "EXPENSE_UPDATE"
	AuditDepartmentCreate = "DEPARTMENT_CREATE"
	AuditDepartmentUpdate = "DEPARTMENT_UPDATE"
	AuditDepartmentDelete = "DEPARTMENT_DELETE"
	AuditAccountCreate    = "ACCOUNT_CREATE"
	AuditAccountUpdate    = "ACCOUNT_UPDATE"
	AuditAccountDelete    = "ACCOUNT_DELETE"
	AuditUserCreate       = "USER_CREATE"
	AuditUserUpdate       = "USER_UPDATE"
	AuditUserDelete       = "USER_DELETE"
	AuditBatchReminder    = "BATCH_REMINDER_RUN"
	AuditCSVExport        = "CSV_EXPORT"
)

// AuditExpenseTransition returns the audit action for a status change, e.g. EXPENSE_SUBMIT.
func AuditExpenseTransition(a domain.Action) string {
	return "EXPENSE_" + strings.ToUpper(string(a))
}

// Meta carries request metadata recorded in the audit log.
type Meta struct {
	IP        string
	UserAgent string
}

// Service holds dependencies for all use cases.
type Service struct {
	Store *store.Store
	// Now returns the current time; replaceable in tests and seeding.
	Now func() time.Time
	// Location is the business timezone used for dates in reports.
	Location *time.Location
	// Mailer receives the dummy reminder e-mails.
	Mailer io.Writer
	// BcryptCost is the cost for password hashing.
	BcryptCost int
	// SessionTTL is how long a login session stays valid.
	SessionTTL time.Duration
	Logger     *slog.Logger
}

// New creates a Service with production defaults.
func New(st *store.Store, logger *slog.Logger, mailer io.Writer) *Service {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.UTC
	}
	return &Service{
		Store:      st,
		Now:        time.Now,
		Location:   loc,
		Mailer:     mailer,
		BcryptCost: bcrypt.DefaultCost,
		SessionTTL: 12 * time.Hour,
		Logger:     logger,
	}
}

func (s *Service) now() string { return store.FormatTime(s.Now()) }

func toJSON(v any) *string {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		str := fmt.Sprintf(`{"marshalError":%q}`, err.Error())
		return &str
	}
	str := string(b)
	return &str
}

// audit appends an audit record on the given transaction.
func (s *Service) audit(ctx context.Context, tx store.DBTX, meta Meta, actorID, action, resourceType, resourceID string, before, after any) error {
	return store.AppendAudit(ctx, tx, store.AuditLog{
		OccurredAt:   s.now(),
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		BeforeJSON:   toJSON(before),
		AfterJSON:    toJSON(after),
		IPAddress:    meta.IP,
		UserAgent:    meta.UserAgent,
	})
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// conflictOn translates DB constraint violations into a domain conflict with a user-facing message.
func conflictOn(err error, msg string) error {
	if store.IsConstraintError(err) {
		return fmt.Errorf("%w: %s", domain.ErrConflict, msg)
	}
	return err
}

func badRequest(msg string) error {
	return fmt.Errorf("%w: %s", domain.ErrBadRequest, msg)
}

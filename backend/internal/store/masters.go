package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
)

// Department is a row of the department master.
type Department struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Account is a row of the account (勘定科目) master.
type Account struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	LimitAmount *int64 `json:"limitAmount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// User is a row of the user master. PasswordHash is never serialized.
type User struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	DepartmentID   int64       `json:"departmentId"`
	DepartmentName string      `json:"departmentName"`
	Role           domain.Role `json:"role"`
	PasswordHash   string      `json:"-"`
	CreatedAt      string      `json:"createdAt"`
	UpdatedAt      string      `json:"updatedAt"`
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- Departments ----

// ListDepartments returns all departments ordered by code.
func ListDepartments(ctx context.Context, q DBTX) ([]Department, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, code, name, created_at, updated_at FROM departments ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Department{}
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDepartment loads one department.
func GetDepartment(ctx context.Context, q DBTX, id int64) (Department, error) {
	var d Department
	err := q.QueryRowContext(ctx, `SELECT id, code, name, created_at, updated_at FROM departments WHERE id = ?`, id).
		Scan(&d.ID, &d.Code, &d.Name, &d.CreatedAt, &d.UpdatedAt)
	return d, notFound(err)
}

// InsertDepartment creates a department and returns its ID.
func InsertDepartment(ctx context.Context, q DBTX, code, name, now string) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO departments (code, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, code, name, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDepartment updates a department.
func UpdateDepartment(ctx context.Context, q DBTX, id int64, code, name, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE departments SET code = ?, name = ?, updated_at = ? WHERE id = ?`, code, name, now, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// DeleteDepartment deletes a department (fails if referenced).
func DeleteDepartment(ctx context.Context, q DBTX, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM departments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// ---- Accounts ----

const accountCols = `id, code, name, active, limit_amount, created_at, updated_at`

func scanAccount(s interface{ Scan(...any) error }) (Account, error) {
	var a Account
	err := s.Scan(&a.ID, &a.Code, &a.Name, &a.Active, &a.LimitAmount, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// ListAccounts returns accounts ordered by code; inactive ones only if includeInactive.
func ListAccounts(ctx context.Context, q DBTX, includeInactive bool) ([]Account, error) {
	query := `SELECT ` + accountCols + ` FROM accounts`
	if !includeInactive {
		query += ` WHERE active = 1`
	}
	rows, err := q.QueryContext(ctx, query+` ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Account{}
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAccount loads one account.
func GetAccount(ctx context.Context, q DBTX, id int64) (Account, error) {
	a, err := scanAccount(q.QueryRowContext(ctx, `SELECT `+accountCols+` FROM accounts WHERE id = ?`, id))
	return a, notFound(err)
}

// AccountRules returns validation rules for all accounts keyed by ID.
func AccountRules(ctx context.Context, q DBTX) (map[int64]domain.AccountRule, error) {
	accs, err := ListAccounts(ctx, q, true)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]domain.AccountRule, len(accs))
	for _, a := range accs {
		out[a.ID] = domain.AccountRule{ID: a.ID, Name: a.Name, Active: a.Active, LimitAmount: a.LimitAmount}
	}
	return out, nil
}

// InsertAccount creates an account and returns its ID.
func InsertAccount(ctx context.Context, q DBTX, a Account, now string) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO accounts (code, name, active, limit_amount, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.Code, a.Name, a.Active, a.LimitAmount, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAccount updates an account.
func UpdateAccount(ctx context.Context, q DBTX, a Account, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE accounts SET code = ?, name = ?, active = ?, limit_amount = ?, updated_at = ? WHERE id = ?`,
		a.Code, a.Name, a.Active, a.LimitAmount, now, a.ID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// DeleteAccount deletes an account (fails if referenced by expense items).
func DeleteAccount(ctx context.Context, q DBTX, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// ---- Users ----

const userSelect = `SELECT u.id, u.name, u.department_id, d.name, u.role, u.password_hash, u.created_at, u.updated_at
FROM users u JOIN departments d ON d.id = u.department_id`

func scanUser(s interface{ Scan(...any) error }) (User, error) {
	var u User
	err := s.Scan(&u.ID, &u.Name, &u.DepartmentID, &u.DepartmentName, &u.Role, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// ListUsers returns all users ordered by ID.
func ListUsers(ctx context.Context, q DBTX) ([]User, error) {
	return queryUsers(ctx, q, userSelect+` ORDER BY u.id`)
}

// ListUsersByRole returns users with the role, optionally restricted to a department (deptID > 0).
func ListUsersByRole(ctx context.Context, q DBTX, role domain.Role, deptID int64) ([]User, error) {
	if deptID > 0 {
		return queryUsers(ctx, q, userSelect+` WHERE u.role = ? AND u.department_id = ? ORDER BY u.id`, role, deptID)
	}
	return queryUsers(ctx, q, userSelect+` WHERE u.role = ? ORDER BY u.id`, role)
}

func queryUsers(ctx context.Context, q DBTX, query string, args ...any) ([]User, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUser loads one user.
func GetUser(ctx context.Context, q DBTX, id string) (User, error) {
	u, err := scanUser(q.QueryRowContext(ctx, userSelect+` WHERE u.id = ?`, id))
	return u, notFound(err)
}

// InsertUser creates a user.
func InsertUser(ctx context.Context, q DBTX, u User, now string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO users (id, name, department_id, role, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.DepartmentID, u.Role, u.PasswordHash, now, now)
	return err
}

// UpdateUser updates a user's profile and password hash.
func UpdateUser(ctx context.Context, q DBTX, u User, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE users SET name = ?, department_id = ?, role = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		u.Name, u.DepartmentID, u.Role, u.PasswordHash, now, u.ID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// DeleteUser deletes a user (fails if referenced by expenses).
func DeleteUser(ctx context.Context, q DBTX, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// ---- Sessions ----

// InsertSession stores a session token hash.
func InsertSession(ctx context.Context, q DBTX, tokenHash, userID, expiresAt, now string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, tokenHash, userID, expiresAt, now)
	return err
}

// SessionUser returns the user for a non-expired session.
func SessionUser(ctx context.Context, q DBTX, tokenHash, now string) (User, error) {
	u, err := scanUser(q.QueryRowContext(ctx, userSelect+` JOIN sessions s ON s.user_id = u.id WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now))
	return u, notFound(err)
}

// DeleteSession removes a session.
func DeleteSession(ctx context.Context, q DBTX, tokenHash string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteExpiredSessions purges expired sessions.
func DeleteExpiredSessions(ctx context.Context, q DBTX, now string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now)
	return err
}

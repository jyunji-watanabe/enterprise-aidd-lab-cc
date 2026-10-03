package domain

// Role is a user's role for role-based access control.
type Role string

// Supported roles.
const (
	RoleEmployee Role = "Employee"
	RoleManager  Role = "Manager"
	RoleAdmin    Role = "Admin"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleEmployee, RoleManager, RoleAdmin:
		return true
	}
	return false
}

// Permission is a coarse-grained capability that is granted to roles.
type Permission string

// Permissions for features beyond the user's own expenses.
const (
	PermManageMasters   Permission = "manage_masters"
	PermViewAllExpenses Permission = "view_all_expenses"
	PermViewAuditLog    Permission = "view_audit_log"
	PermExportReports   Permission = "export_reports"
	PermRunBatch        Permission = "run_batch"
	PermApproveDept     Permission = "approve_department"
	PermSettle          Permission = "settle"
)

var rolePermissions = map[Role]map[Permission]bool{
	RoleEmployee: {},
	RoleManager: {
		PermApproveDept: true,
	},
	RoleAdmin: {
		PermManageMasters:   true,
		PermViewAllExpenses: true,
		PermViewAuditLog:    true,
		PermExportReports:   true,
		PermRunBatch:        true,
		PermSettle:          true,
	},
}

// Can reports whether the role is granted the permission.
func (r Role) Can(p Permission) bool {
	return rolePermissions[r][p]
}

// Actor is the authenticated user performing an operation.
type Actor struct {
	UserID       string
	Name         string
	Role         Role
	DepartmentID int64
}

// Require returns ErrForbidden unless the actor's role has the permission.
func (a Actor) Require(p Permission) error {
	if !a.Role.Can(p) {
		return ErrForbidden
	}
	return nil
}

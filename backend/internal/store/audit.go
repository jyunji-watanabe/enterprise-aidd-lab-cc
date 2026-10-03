package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

// GenesisHash is the prev_hash of the first audit record.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// AuditLog is one immutable audit record.
type AuditLog struct {
	ID           int64   `json:"id"`
	OccurredAt   string  `json:"occurredAt"`
	ActorID      string  `json:"actorId"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	BeforeJSON   *string `json:"before"`
	AfterJSON    *string `json:"after"`
	IPAddress    string  `json:"ipAddress"`
	UserAgent    string  `json:"userAgent"`
	PrevHash     string  `json:"prevHash"`
	Hash         string  `json:"hash"`
}

func deref(s *string) string {
	if s == nil {
		return "\x00"
	}
	return *s
}

// ComputeHash returns the chain hash of the record (excluding ID and Hash).
func (l AuditLog) ComputeHash() string {
	parts := []string{l.PrevHash, l.OccurredAt, l.ActorID, l.Action, l.ResourceType, l.ResourceID,
		deref(l.BeforeJSON), deref(l.AfterJSON), l.IPAddress, l.UserAgent}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// AppendAudit appends a record, linking it to the previous record's hash.
// It must run on the same transaction as the change it audits.
func AppendAudit(ctx context.Context, q DBTX, l AuditLog) error {
	err := q.QueryRowContext(ctx, `SELECT hash FROM audit_logs ORDER BY id DESC LIMIT 1`).Scan(&l.PrevHash)
	if errors.Is(err, sql.ErrNoRows) {
		l.PrevHash = GenesisHash
	} else if err != nil {
		return err
	}
	l.Hash = l.ComputeHash()
	_, err = q.ExecContext(ctx, `INSERT INTO audit_logs (occurred_at, actor_id, action, resource_type, resource_id, before_json, after_json, ip_address, user_agent, prev_hash, hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.OccurredAt, l.ActorID, l.Action, l.ResourceType, l.ResourceID, l.BeforeJSON, l.AfterJSON, l.IPAddress, l.UserAgent, l.PrevHash, l.Hash)
	return err
}

// AuditFilter narrows ListAudit.
type AuditFilter struct {
	Action       string
	ActorID      string
	ResourceType string
	Limit        int
	Offset       int
}

const auditCols = `id, occurred_at, actor_id, action, resource_type, resource_id, before_json, after_json, ip_address, user_agent, prev_hash, hash`

func scanAudit(s interface{ Scan(...any) error }) (AuditLog, error) {
	var l AuditLog
	err := s.Scan(&l.ID, &l.OccurredAt, &l.ActorID, &l.Action, &l.ResourceType, &l.ResourceID, &l.BeforeJSON, &l.AfterJSON, &l.IPAddress, &l.UserAgent, &l.PrevHash, &l.Hash)
	return l, err
}

// ListAudit returns audit records newest first plus the total matching count.
func ListAudit(ctx context.Context, q DBTX, f AuditFilter) ([]AuditLog, int, error) {
	var where []string
	var args []any
	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, f.Action)
	}
	if f.ActorID != "" {
		where = append(where, "actor_id = ?")
		args = append(args, f.ActorID)
	}
	if f.ResourceType != "" {
		where = append(where, "resource_type = ?")
		args = append(args, f.ResourceType)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := q.QueryContext(ctx, `SELECT `+auditCols+` FROM audit_logs`+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []AuditLog{}
	for rows.Next() {
		l, err := scanAudit(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// VerifyAuditChain recomputes the hash chain. It returns the number of
// records checked and, if tampering is detected, the ID of the first bad record.
func VerifyAuditChain(ctx context.Context, q DBTX) (checked int, brokenAt int64, err error) {
	rows, err := q.QueryContext(ctx, `SELECT `+auditCols+` FROM audit_logs ORDER BY id`)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	prev := GenesisHash
	for rows.Next() {
		l, err := scanAudit(rows)
		if err != nil {
			return checked, 0, err
		}
		checked++
		if l.PrevHash != prev || l.ComputeHash() != l.Hash {
			return checked, l.ID, nil
		}
		prev = l.Hash
	}
	return checked, 0, rows.Err()
}

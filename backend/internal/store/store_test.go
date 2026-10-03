package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func appendN(t *testing.T, st *Store, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		after := `{"i":1}`
		err := AppendAudit(ctx, st.DB, AuditLog{OccurredAt: FormatTime(time.Now()), ActorID: "u", Action: "TEST", ResourceType: "r", ResourceID: "1", AfterJSON: &after})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	st := openTest(t)
	if err := st.migrate(context.Background()); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	st := openTest(t)
	appendN(t, st, 2)
	ctx := context.Background()
	if _, err := st.DB.ExecContext(ctx, `UPDATE audit_logs SET actor_id = 'evil'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE must be rejected, got %v", err)
	}
	if _, err := st.DB.ExecContext(ctx, `DELETE FROM audit_logs`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE must be rejected, got %v", err)
	}
}

func TestAuditHashChain(t *testing.T) {
	st := openTest(t)
	appendN(t, st, 3)
	ctx := context.Background()
	n, broken, err := VerifyAuditChain(ctx, st.DB)
	if err != nil || n != 3 || broken != 0 {
		t.Fatalf("chain should be valid: n=%d broken=%d err=%v", n, broken, err)
	}

	// Simulate tampering that bypasses the triggers (e.g. editing the file offline).
	if _, err := st.DB.ExecContext(ctx, `DROP TRIGGER audit_logs_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.ExecContext(ctx, `UPDATE audit_logs SET actor_id = 'evil' WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	_, broken, err = VerifyAuditChain(ctx, st.DB)
	if err != nil || broken != 2 {
		t.Fatalf("tampering on row 2 must be detected, got broken=%d err=%v", broken, err)
	}
}

func TestWithTxRollsBack(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	err := st.WithTx(ctx, func(tx DBTX) error {
		if _, err := InsertDepartment(ctx, tx, "X", "x", FormatTime(time.Now())); err != nil {
			return err
		}
		_, err := InsertDepartment(ctx, tx, "X", "dup", FormatTime(time.Now()))
		return err
	})
	if !IsConstraintError(err) {
		t.Fatalf("expected unique constraint error, got %v", err)
	}
	ds, _ := ListDepartments(ctx, st.DB)
	if len(ds) != 0 {
		t.Fatalf("transaction must roll back, found %d departments", len(ds))
	}
}

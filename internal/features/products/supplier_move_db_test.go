package products

import (
	"context"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

// SUP-11: Move products and the plugins' rules move in one transaction — a hook
// that fails leaves every product where it was.
func TestSUP11_MoveRollsBackOnHookError(t *testing.T) {
	svc, f := ruleSetup(t)
	SupplierMoveHook = func(context.Context, *sqlx.Tx, int64, int64) error { return errors.New("plugin broke") }
	t.Cleanup(func() { SupplierMoveHook = nil })
	if _, err := svc.ReassignPreferredSupplier(context.Background(), f.s1, f.s2); err == nil {
		t.Fatal("move went through with a failing hook")
	}
	var own int64
	must(t, svc.db.GetContext(context.Background(), &own, `SELECT preferred_supplier_id FROM products WHERE id = $1`, f.a))
	if own != f.s1 {
		t.Errorf("product moved to %d although the hook failed", own)
	}
}
